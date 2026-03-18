# kubernetes-ip-tracker: A Code Walkthrough

*2026-03-18T12:20:51Z by Showboat 0.6.1*
<!-- showboat-id: 5d4720ca-949f-46eb-b710-8fe38b281082 -->

This walkthrough follows the code in execution order — from the data model the CRD is built on, through the controller manager startup, into the reconcile loop, and finally to the node agent DaemonSet running on every worker node. Two independent Go programs cooperate to fill a single Kubernetes status object.

## Step 1: The Data Model

Everything starts with `api/v1/podtracker_types.go`. This file declares the Go structs that become the CRD schema. Kubebuilder's code-generator reads the marker comments (lines starting with `//+kubebuilder:`) and produces the YAML CRD manifest automatically.

```bash
sed -n '23,112p' api/v1/podtracker_types.go
```

```output
// PodTrackerSpec defines the desired state of PodTracker
type PodTrackerSpec struct {
	// Namespace to track pods in, if empty tracks all namespaces
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PodInfo contains information about a pod
type PodInfo struct {
	// PodName is the name of the pod
	PodName string `json:"podName"`

	// Namespace is the namespace of the pod
	Namespace string `json:"namespace"`

	// IP is the IP address of the pod
	IP string `json:"ip"`

	// NodeName is the name of the node running the pod
	NodeName string `json:"nodeName"`

	// CreationTimestamp is when the pod was created
	CreationTimestamp metav1.Time `json:"creationTimestamp"`

	// Phase is the current lifecycle phase of the pod
	Phase string `json:"phase"`
}

// NodeInfo contains information about a node
type NodeInfo struct {
	// NodeName is the name of the node
	NodeName string `json:"nodeName"`

	// NodeIP is the primary IP address of the node
	NodeIP string `json:"nodeIP"`

	// PodIPs lists all pods running on this node
	PodIPs []PodInfo `json:"podIPs,omitempty"`

	// Resources contains node resource information
	Resources NodeResources `json:"resources,omitempty"`

	// LastUpdateTime is when this node was last updated
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
}

// NodeResources contains resource information about a node
type NodeResources struct {
	// CPU capacity
	CPUCapacity string `json:"cpuCapacity"`

	// Memory capacity
	MemoryCapacity string `json:"memoryCapacity"`

	// CPU allocatable
	CPUAllocatable string `json:"cpuAllocatable"`

	// Memory allocatable
	MemoryAllocatable string `json:"memoryAllocatable"`
}

// Update the PodTrackerStatus
type PodTrackerStatus struct {
	// PodIPs contains a list of all tracked pods and their IPs
	// +optional
	PodIPs []PodInfo `json:"podIPs,omitempty"`

	// NodeInfo contains node-specific information
	// +optional
	NodeInfo []NodeInfo `json:"nodeInfo,omitempty"`

	// LastUpdateTime is the last time the resource was updated
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
//+kubebuilder:printcolumn:name="Pods",type=integer,JSONPath=`.status.podIPs.length()`

// PodTracker is the Schema for the podtrackers API
type PodTracker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodTrackerSpec   `json:"spec,omitempty"`
	Status PodTrackerStatus `json:"status,omitempty"`
}
```

The spec is intentionally minimal — just one optional field, `namespace`. All the interesting data lives in the **status**, which is split into two halves:

- `status.podIPs` — a flat list of every `PodInfo` across the cluster, written by the central **controller**
- `status.nodeInfo` — a list of `NodeInfo` entries (one per node), each containing pods and hardware resources, written by the per-node **agent**

This split is deliberate: the two programs own non-overlapping parts of the same status object, so they can write independently without stepping on each other.

The `//+kubebuilder:` marker comments at the bottom drive code generation: `scope=Cluster` makes the CRD cluster-scoped (no namespace), `subresource:status` enables a dedicated `/status` endpoint so spec and status can be updated with separate RBAC, and `printcolumn` controls what `kubectl get podtrackers` displays.

## Step 2: Registering the API Group

`api/v1/groupversion_info.go` declares the API group identity and wires the Go types into the Kubernetes runtime scheme. This is the bridge between the Go struct names and the `apiVersion`/`kind` strings in YAML manifests.

```bash
sed -n '17,36p' api/v1/groupversion_info.go
```

```output
// Package v1 contains API Schema definitions for the networking v1 API group.
// +kubebuilder:object:generate=true
// +groupName=networking.learntosolveit.com
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "networking.learntosolveit.com", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
```

```bash
sed -n '123,125p' api/v1/podtracker_types.go
```

```output
func init() {
	SchemeBuilder.Register(&PodTracker{}, &PodTrackerList{})
}
```

The `init()` in `podtracker_types.go` calls `SchemeBuilder.Register`, which records that the Go type `PodTracker` maps to `networking.learntosolveit.com/v1/PodTracker`. Then in `cmd/main.go`, `networkingv1.AddToScheme(scheme)` installs those mappings into the runtime scheme used by the manager and client. Without this chain, the API server and the client library would not know how to serialize or deserialize `PodTracker` objects.

## Step 3: The Controller Manager Entry Point

`cmd/main.go` is where the controller binary starts. It wires together the scheme, the manager, the reconciler, and the health endpoints.

```bash
sed -n '39,49p' cmd/main.go
```

```output
var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(networkingv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}
```

The scheme is a global that gets two sets of types added in `init()`: the standard Kubernetes types (pods, nodes, events, …) via `clientgoscheme.AddToScheme`, and the custom `PodTracker` types via `networkingv1.AddToScheme`. Both must be present before the manager starts — the scheme is how the client knows which Go struct to decode any given API response into.

```bash
sed -n '51,117p' cmd/main.go
```

```output
func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "c1b5a7d4.learntosolveit.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.PodTrackerReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("podtracker-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodTracker")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
```

`ctrl.NewManager` creates the controller-runtime manager — the central coordinator that owns the shared Kubernetes client, an in-memory cache (informers), and the work queue. Passing the `scheme` here makes the shared cache type-aware.

The reconciler is constructed inline and handed three things: the shared `Client` (cache-backed reads, direct writes), the `Scheme`, and an `EventRecorder` so it can emit Kubernetes events. `SetupWithManager` registers which resource types the reconciler watches — we'll look at that next.

Finally, `mgr.Start(ctrl.SetupSignalHandler())` blocks until SIGTERM/SIGINT. The signal handler gives the manager a chance to drain in-flight reconciliations before exiting.

## Step 4: Registering Watches — What Triggers Reconciliation

`SetupWithManager` in `internal/controller/podtracker_controller.go` tells the manager which objects to watch and how to translate change events into reconcile requests.

```bash
sed -n '152,203p' internal/controller/podtracker_controller.go
```

```output
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
```

Two watches are registered:

1. **`For(&networkingv1.PodTracker{})`** — any create/update/delete on a `PodTracker` object queues a reconcile for that same object. This is the standard primary watch.

2. **`Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.findAllPodTrackers))`** — any change to any Pod in the cluster runs `findAllPodTrackers`. That function lists all `PodTracker` resources, filters out any whose `spec.namespace` doesn't match the changed pod's namespace, and returns a reconcile request for each remaining one.

The result: any pod lifecycle event (IP assigned, pod deleted, pod restarted) immediately triggers a reconcile of every relevant `PodTracker` — on top of the periodic 5-minute requeue that the reconcile loop schedules for itself.

## Step 5: The Reconcile Loop

The `Reconcile` method is the core of the controller. Controller-runtime calls it whenever the work queue has a request for a `PodTracker` name. It must be idempotent — the same result whether called once or a hundred times.

```bash
sed -n '57,94p' internal/controller/podtracker_controller.go
```

```output
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
```

**Phase 1 — Fetch.** The reconciler immediately re-fetches the current state of the `PodTracker` from the API server. If it's already gone (deleted between the event firing and now), return quietly. This is the standard Kubernetes controller pattern: always work from current state, not from the event that triggered you.

**Phase 2 — Finalizer.** On the very first reconcile of a new `PodTracker`, the finalizer string `podtracker.networking.learntosolveit.com/finalizer` is missing. The reconciler adds it and returns immediately — this causes another reconcile where the finalizer is already present and normal processing begins.

The finalizer is the mechanism that lets the controller run cleanup logic on deletion. Kubernetes won't actually delete the resource until all finalizers are removed. Here, deletion handling is trivial (just remove the finalizer), but the pattern is in place if cleanup logic needs to be added later.

```bash
sed -n '96,150p' internal/controller/podtracker_controller.go
```

```output
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
```

**Phase 3 — List pods.** If `spec.namespace` is set, the list is scoped to that namespace; otherwise it lists pods across the entire cluster. Pods with no `PodIP` yet (still pending) are skipped — only running pods with assigned IPs are recorded.

**Phase 4 — Cooperative status write.** This is the key design decision. Before writing:

```go
existingNodeInfo := podTracker.Status.NodeInfo
podTracker.Status.PodIPs = podIPs
podTracker.Status.NodeInfo = existingNodeInfo  // put it back
```

The controller snapshots `nodeInfo` (written by the agents), overwrites `podIPs`, then restores `nodeInfo`. This prevents the controller from accidentally zeroing out agent data. The agents do the symmetric thing when they update their node's entry.

The write only happens at all if `reflect.DeepEqual` says the new pod list differs from the stored one — avoiding a pointless API call and a spurious resource-version bump on every periodic requeue.

After writing (or skipping), the reconcile returns `RequeueAfter: 5 minutes` — a safety net that ensures the state is refreshed even if no pod events fire.

## Step 6: The Node Agent

`cmd/agent/main.go` is an entirely separate binary, deployed as a DaemonSet so one instance runs on each worker node. It does not use controller-runtime's reconcile loop — it runs a plain polling loop.

```bash
sed -n '21,50p' cmd/agent/main.go
```

```output
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

```

The agent creates its own Kubernetes client using `rest.InClusterConfig()` — the credentials are mounted automatically by Kubernetes into every pod via a ServiceAccount token. Crucially it creates a **direct** `client.New` client, not a manager-backed cache client. Reads go straight to the API server, which is fine for a polling loop but means there's no shared informer cache reducing API server load.

`NODE_NAME` is injected by the DaemonSet manifest using the Kubernetes downward API — the pod can read its own scheduling node without any privileged access.

```bash
grep -A4 'NODE_NAME' config/agent/daemonset.yaml
```

```output
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
```

The `fieldRef: spec.nodeName` is the downward API in action. Kubernetes populates this env var at pod scheduling time with the name of the node the pod landed on — so each DaemonSet pod automatically knows its own node identity without any runtime discovery.

```bash
sed -n '52,117p' cmd/agent/main.go
```

```output
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
```

The agent's loop does three things per iteration:

1. **Fetch the node object** to get its IP (walks `node.Status.Addresses` to find the `NodeInternalIP` type) and resource totals from `node.Status.Capacity` and `node.Status.Allocatable`.

2. **List pods on this node only** using a field selector `spec.nodeName=<nodeName>`. Field selectors are evaluated server-side, so only the pods for this specific node are returned over the network — much more efficient than fetching all pods and filtering locally.

3. **Build a `NodeInfo` struct** that packages the node IP, CPU/memory capacity and allocatable amounts, and the per-pod details into a single value ready to write.

```bash
sed -n '119,167p' cmd/agent/main.go
```

```output
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
```

**Writing the agent's data — optimistic concurrency.** The agent lists all `PodTracker` resources, then for each one:

1. **Re-fetches the latest version** with `cl.Get` immediately before writing. This is the optimistic concurrency pattern: the list result may be stale by the time we get here, so we fetch fresh to get the latest `resourceVersion`. Kubernetes will reject a status update if the `resourceVersion` in the request doesn't match what's on the server.

2. **Upserts its node entry** — walks `status.nodeInfo` looking for an entry with matching `nodeName`. If found, it replaces in-place; if not (first run, or a new node), it appends. All other nodes' entries are untouched — this is the symmetric cooperative-write pattern.

3. **Calls `cl.Status().Update`** — the `/status` subresource endpoint. Because the CRD was declared with `//+kubebuilder:subresource:status`, spec and status have separate endpoints and separate RBAC. The agent's ServiceAccount only needs `update` on `podtrackers/status`, not on `podtrackers` itself.

After iterating all `PodTracker` resources, the loop sleeps for one minute.

## Step 7: How It All Fits Together — A Request's Journey

Here is the full sequence from a pod starting to the data appearing in the CRD:

```bash
printf "1. A new Pod is scheduled and receives an IP (e.g., 10.244.1.42 on node worker-1).\n\n2. The controller-runtime informer for Pod objects detects the change and\n   calls findAllPodTrackers, which enqueues a reconcile request for each\n   PodTracker whose namespace filter matches.\n\n3. Reconcile() fires: lists pods, builds updated podIPs slice, snapshots\n   nodeInfo, writes status.podIPs, restores nodeInfo.\n\n4. Independently, the agent on worker-1 wakes, collects node+pod info,\n   re-fetches each PodTracker (fresh resourceVersion), upserts its nodeInfo\n   entry, and writes status.nodeInfo.\n\n5. The PodTracker status now reflects the pod in both sections:\n   - status.podIPs        (global flat view, from controller)\n   - status.nodeInfo[worker-1].podIPs  (per-node view, from agent)"
```

```output
1. A new Pod is scheduled and receives an IP (e.g., 10.244.1.42 on node worker-1).

2. The controller-runtime informer for Pod objects detects the change and
   calls findAllPodTrackers, which enqueues a reconcile request for each
   PodTracker whose namespace filter matches.

3. Reconcile() fires: lists pods, builds updated podIPs slice, snapshots
   nodeInfo, writes status.podIPs, restores nodeInfo.

4. Independently, the agent on worker-1 wakes, collects node+pod info,
   re-fetches each PodTracker (fresh resourceVersion), upserts its nodeInfo
   entry, and writes status.nodeInfo.

5. The PodTracker status now reflects the pod in both sections:
   - status.podIPs        (global flat view, from controller)
   - status.nodeInfo[worker-1].podIPs  (per-node view, from agent)```
```

## Step 8: Deploying a PodTracker — The User-Facing API

To activate tracking you apply a single CR:

```bash
cat config/samples/networking_v1_podtracker.yaml
```

```output
apiVersion: networking.learntosolveit.com/v1
kind: PodTracker
metadata:
  name: cluster-pod-tracker
spec:
  # Leave namespace empty to track all namespaces
  namespace: ""```
```

That is the entire configuration surface. Setting `spec.namespace: "kube-system"` would scope both the controller's pod listing and the pod-event filtering to just that namespace. Leaving it empty (or omitting it) watches all namespaces cluster-wide.

The CRD is cluster-scoped (`scope=Cluster`), so the object has no namespace itself — it can see across the whole cluster regardless of where it lives in the API.

## Step 9: RBAC — What Each Component Is Allowed To Do

The RBAC comments in the controller file are not just documentation — Kubebuilder's `make manifests` command reads them and generates the actual RBAC YAML in `config/rbac/`.

```bash
sed -n '50,54p' internal/controller/podtracker_controller.go
```

```output
//+kubebuilder:rbac:groups=networking.learntosolveit.com,resources=podtrackers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.learntosolveit.com,resources=podtrackers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.learntosolveit.com,resources=podtrackers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
```

The controller needs full CRUD on `podtrackers` (to add/remove finalizers via `r.Update`) and write access to `podtrackers/status` (to call `r.Status().Update`). Separating these into different RBAC rules is made possible by the status subresource declared in the CRD markers.

Read-only access to `pods` is sufficient for the controller — it only lists them to build the IP snapshot, never writes to them.

The node agent has a separate ServiceAccount (`pod-ip-tracker-agent`) with tighter permissions: it only needs `get;list;watch` on nodes and pods, plus `get;update;patch` on `podtrackers/status`. It never touches the `podtrackers` resource itself.

## Summary: The Two-Writer Architecture

The design can be summarised as two writers, one status object, non-overlapping fields:

| Writer | Deployment | Trigger | Writes to |
|--------|-----------|---------|-----------|
| Controller | Deployment (singleton) | Pod events + 5 min requeue | `status.podIPs` |
| Agent | DaemonSet (one per node) | 60 second poll | `status.nodeInfo[thisNode]` |

Each writer snapshots and restores the other's section before its own `Status().Update` call. Because Kubernetes uses optimistic concurrency (resourceVersion check), a write conflict causes an error — the losing writer simply retries on its next cycle rather than implementing explicit locking.
