package main

import (
	"context"
	"k8s.io/apimachinery/pkg/fields"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1 "github.com/orsenthil/kubernetes-ip-tracker/api/v1"
)

func main() {
	klog.InitFlags(nil)

	// Create a scheme and register our types
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))

	// Get in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Error getting Kubernetes config: %v", err)
	}

	// Create client
	cl, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		klog.Fatalf("Error creating client: %v", err)
	}

	// Get node name from environment
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Fatalf("NODE_NAME environment variable not set")
	}

	klog.Infof("Starting PodTracker node agent on node: %s", nodeName)

	// Main loop to collect and update node information
	for {
		ctx := context.Background()

		// Get the node information
		node := &corev1.Node{}
		err := cl.Get(ctx, client.ObjectKey{Name: nodeName}, node)
		if err != nil {
			klog.Errorf("Error getting node %s: %v", nodeName, err)
			time.Sleep(time.Minute)
			continue
		}

		// List pods on this node
		fieldSelector := fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName})
		podList := &corev1.PodList{}
		err = cl.List(ctx, podList, &client.ListOptions{
			FieldSelector: fieldSelector,
		})

		if err != nil {
			klog.Errorf("Error listing pods on node %s: %v", nodeName, err)
			time.Sleep(time.Minute)
			continue
		}

		// Get node IP address
		nodeIP := ""
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				nodeIP = addr.Address
				break
			}
		}

		// Collect pod information for this node
		podInfos := []networkingv1.PodInfo{}
		for _, pod := range podList.Items {
			if pod.Status.PodIP == "" {
				continue // Skip pods without IPs
			}

			podInfo := networkingv1.PodInfo{
				PodName:           pod.Name,
				Namespace:         pod.Namespace,
				IP:                pod.Status.PodIP,
				NodeName:          nodeName,
				CreationTimestamp: pod.CreationTimestamp,
				Phase:             string(pod.Status.Phase),
			}

			podInfos = append(podInfos, podInfo)
		}

		// Prepare node info
		nodeInfo := networkingv1.NodeInfo{
			NodeName: nodeName,
			NodeIP:   nodeIP,
			Resources: networkingv1.NodeResources{
				CPUCapacity:       node.Status.Capacity.Cpu().String(),
				MemoryCapacity:    node.Status.Capacity.Memory().String(),
				CPUAllocatable:    node.Status.Allocatable.Cpu().String(),
				MemoryAllocatable: node.Status.Allocatable.Memory().String(),
			},
			PodIPs:         podInfos,
			LastUpdateTime: metav1.Now(),
		}

		// Get all podtrackers
		podTrackerList := &networkingv1.PodTrackerList{}
		err = cl.List(ctx, podTrackerList)
		if err != nil {
			klog.Errorf("Error listing PodTrackers: %v", err)
			time.Sleep(time.Minute)
			continue
		}

		for i := range podTrackerList.Items {
			podTracker := &podTrackerList.Items[i]

			// First, fetch the latest version to avoid conflicts
			currentPodTracker := &networkingv1.PodTracker{}
			err = cl.Get(ctx, client.ObjectKey{Name: podTracker.Name}, currentPodTracker)
			if err != nil {
				klog.Errorf("Error getting latest PodTracker %s: %v", podTracker.Name, err)
				continue
			}

			// Update the podTracker with this node's information
			updated := false
			for j, existingNode := range currentPodTracker.Status.NodeInfo {
				if existingNode.NodeName == nodeInfo.NodeName {
					currentPodTracker.Status.NodeInfo[j] = nodeInfo
					updated = true
					break
				}
			}

			// Add new node if not found
			if !updated {
				currentPodTracker.Status.NodeInfo = append(currentPodTracker.Status.NodeInfo, nodeInfo)
			}

			currentPodTracker.Status.LastUpdateTime = metav1.Now()

			// Update the PodTracker status
			err = cl.Status().Update(ctx, currentPodTracker)
			if err != nil {
				klog.Errorf("Error updating PodTracker %s: %v", currentPodTracker.Name, err)
			} else {
				klog.Infof("Successfully updated PodTracker %s with node info for %s", currentPodTracker.Name, nodeInfo.NodeName)
			}
		}

		// Sleep for a while before next update
		time.Sleep(time.Minute)
	}
}
