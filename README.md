# kubernetes-ip-tracker
Kubernetes Pod IP tracker

# Setup the kubebuilder

```bash
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder
sudo mv kubebuilder /usr/local/bin/

# Verify installation
kubebuilder version
```
# Create the project

```bash
# Create project directory
mkdir -p $HOME/kubernetes-ip-tracker
cd $HOME/kubernetes-ip-tracker

# Initialize a new project with Kubebuilder
go mod init github.com/orsenthil/kubernetes-ip-tracker
kubebuilder init --domain learntosolveit.com --repo github.com/orsenthil/kubernetes-ip-tracker
```

# Create the API types and controller scaffold

```bash
# Create API types and controller scaffold
kubebuilder create api --group networking --version v1 --kind PodTracker --resource --controller
```

# Code 

* Define pod tracker in

```shell
api/v1/podtracker_types.go
```

* Define the controller in
 
```shell
controllers/podtracker_controller.go
```

* Update main

```shell
main.go
```

# Create a Sample Pod Tracker Custom Resource

```shell
mkdir -p config/samples
vi config/samples/networking_v1_podtracker.yaml
```

# Generate CRDs, Manifest and RBAC

```shell
# Generate CRD manifests
make manifests
```

This will create the CRD definition in `config/crd/bases/networking.learntosolveit.com_podtrackers.yaml`


# Build Controller
```shell
make build
```

## Install CRDs to the cluster

```shell
# Install CRDs to the cluster
make install
```

## Build and Deploy Image

```shell
export IMG=docker.io/skumaran/kubernetes-ip-tracker:v0.1.0  # For Docker Hub
make docker-build
make deploy IMG=$IMG
```

## Verify the Deployments

```shell
kubectl get pods -n kubernetes-ip-tracker-system
```

```shell
kubectl apply -f config/samples/networking_v1_podtracker.yaml
```

* Verify the Pod Tracker Works

```shell
# List all podtrackers
kubectl get podtrackers

# Get detailed information of our PodTracker
kubectl describe podtracker cluster-pod-tracker

# View the collected pod IPs
kubectl get podtracker cluster-pod-tracker -o jsonpath='{.status.podIPs}' | jq
```

* Check the controller logs
 
```shell
kubectl logs -n kubernetes-ip-tracker-system -l control-plane=controller-manager -c manager
```