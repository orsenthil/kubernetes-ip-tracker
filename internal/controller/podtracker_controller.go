/*
Copyright 2025 Senthil Kumaran.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"reflect"
	"time"

	networkingv1 "github.com/orsenthil/kubernetes-ip-tracker/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PodTrackerReconciler reconciles a PodTracker object
type PodTrackerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const (
	podTrackerFinalizer = "podtracker.networking.learntosolveit.com/finalizer"
)

//+kubebuilder:rbac:groups=networking.learntosolveit.com,resources=podtrackers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.learntosolveit.com,resources=podtrackers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.learntosolveit.com,resources=podtrackers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *PodTrackerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling PodTracker", "name", req.Name)

	// Fetch the PodTracker instance
	podTracker := &networkingv1.PodTracker{}
	err := r.Get(ctx, req.NamespacedName, podTracker)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request
		return ctrl.Result{}, err
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(podTracker, podTrackerFinalizer) {
		controllerutil.AddFinalizer(podTracker, podTrackerFinalizer)
		if err := r.Update(ctx, podTracker); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		logger.Info("Added finalizer to PodTracker")
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if !podTracker.ObjectMeta.DeletionTimestamp.IsZero() {
		// Resource is being deleted
		controllerutil.RemoveFinalizer(podTracker, podTrackerFinalizer)
		if err := r.Update(ctx, podTracker); err != nil {
			logger.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		logger.Info("Removed finalizer from PodTracker")
		return ctrl.Result{}, nil
	}

	// List all pods in the cluster or specific namespace
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{}

	if podTracker.Spec.Namespace != "" {
		listOpts = append(listOpts, client.InNamespace(podTracker.Spec.Namespace))
	}

	if err := r.List(ctx, podList, listOpts...); err != nil {
		logger.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	// Build list of pod IPs
	podIPs := make([]networkingv1.PodInfo, 0, len(podList.Items))
	for _, pod := range podList.Items {
		// Skip pods without IPs (not running yet)
		if pod.Status.PodIP == "" {
			continue
		}

		podIPs = append(podIPs, networkingv1.PodInfo{
			PodName:           pod.Name,
			Namespace:         pod.Namespace,
			IP:                pod.Status.PodIP,
			NodeName:          pod.Spec.NodeName,
			CreationTimestamp: pod.CreationTimestamp,
			Phase:             string(pod.Status.Phase),
		})
	}

	// Update status if needed
	if !reflect.DeepEqual(podTracker.Status.PodIPs, podIPs) {
		existingNodeInfo := podTracker.Status.NodeInfo

		// Update the PodIPs and timestamp
		podTracker.Status.PodIPs = podIPs
		podTracker.Status.LastUpdateTime = metav1.NewTime(time.Now())

		// Restore the NodeInfo
		podTracker.Status.NodeInfo = existingNodeInfo

		if err := r.Status().Update(ctx, podTracker); err != nil {
			logger.Error(err, "Failed to update PodTracker status")
			return ctrl.Result{}, err
		}

		logger.Info("Updated PodTracker status", "podCount", len(podIPs))
		r.Recorder.Event(podTracker, corev1.EventTypeNormal, "StatusUpdated",
			"PodTracker status updated with current pod IPs")
	}

	// Requeue to periodically check for changes
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *PodTrackerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.PodTracker{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findAllPodTrackers),
		).
		Complete(r)
}

// findAllPodTrackers returns requests for all PodTracker resources when a pod changes
func (r *PodTrackerReconciler) findAllPodTrackers(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)

	// Extract the pod from the object
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		logger.Error(nil, "Expected a Pod but got something else", "type", reflect.TypeOf(obj))
		return []reconcile.Request{}
	}

	podTrackers := &networkingv1.PodTrackerList{}
	err := r.List(ctx, podTrackers)
	if err != nil {
		logger.Error(err, "Failed to list PodTrackers")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0, len(podTrackers.Items))
	for _, item := range podTrackers.Items {
		// If the PodTracker has a namespace filter, check if the pod is in that namespace
		if item.Spec.Namespace != "" && pod.Namespace != item.Spec.Namespace {
			continue
		}

		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: item.Name,
			},
		})
	}

	if len(requests) > 0 {
		logger.V(1).Info("Pod change triggered reconciliation",
			"pod", pod.Name,
			"namespace", pod.Namespace,
			"ip", pod.Status.PodIP,
			"reconciliationRequests", len(requests))
	}
	return requests
}
