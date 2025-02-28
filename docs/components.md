# System Components

![](./img/img.png)

Kubernetes API Server: The core component of the Kubernetes control plane that exposes the Kubernetes API. All communications in the system flow through the API server.
etcd: The persistent storage where all Kubernetes cluster data is saved, including our PodTracker Custom Resource.
Pod IP Tracker Controller: This is the custom controller we've built that watches for Pod events (creation, updates, deletion) and keeps track of their IP addresses.
PodTracker CRD (Custom Resource Definition): The schema that extends the Kubernetes API, defining a new resource type to store pod IP information.
Cluster Pods: The actual Pods running across your Kubernetes cluster whose IP addresses are being tracked.

Key Workflows
The architecture works through these primary workflows:
1. Pod Event Monitoring

The controller establishes a watch connection to the Kubernetes API Server for Pod resources
When Pods are created/updated/deleted, the API Server sends events to our controller
The controller processes these events and determines if any PodTracker resources need to be updated

2. CRD Status Updates

The controller retrieves the current list of Pods and their IP addresses
It then updates the status section of the PodTracker Custom Resource
The updated information is persisted in etcd via the API Server

3. Information Retrieval

Users and applications can query the PodTracker resource via standard kubectl commands or the Kubernetes API
This provides a consistent, centralized view of all Pod IPs in the cluster or a specific namespace

Technical Implementation
Our implementation has:

A Go-based controller built with controller-runtime that:

Watches for Pod events across the cluster
Maintains finalizers for proper cleanup
Processes Pod information and extracts IP addresses
Updates the PodTracker status with current information


A cluster-scoped CRD that:

Defines the PodTracker resource type
Includes spec fields for configuration (like namespace filtering)
Maintains status fields for Pod IP information
Provides additional metadata like creation timestamps and node placement


Deployment resources that:

Set up proper RBAC permissions
Deploy the controller securely in the cluster
Establish watch connections to the API server



This architecture follows the Kubernetes operator pattern, extending the Kubernetes API with custom resources and controllers to provide specialized functionality in a Kubernetes-native way.