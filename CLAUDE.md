# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Kubernetes operator (built with Kubebuilder) that tracks Pod IP addresses and node resource information across a cluster. It uses a **Custom Resource Definition (CRD)** called `PodTracker` and consists of two components:

1. **Controller Manager** (`cmd/main.go`) — singleton Deployment; reconciles `PodTracker` resources, lists pods, and updates `podIPs` in the CRD status.
2. **Node Agent** (`cmd/agent/main.go`) — DaemonSet (one per node); collects per-node pod/resource info and updates the `nodeInfo` section of the CRD status.

Both components cooperate: the controller writes `podIPs`, the agent writes `nodeInfo`, and each preserves the other's data on updates.

## Commands

```bash
# Code generation (run after changing api/v1/ types)
make manifests     # Regenerate CRD manifests and RBAC
make generate      # Regenerate DeepCopy methods

# Code quality
make fmt           # go fmt
make vet           # go vet
make lint          # golangci-lint run
make lint-fix      # golangci-lint run --fix

# Testing
make test          # Unit tests with envtest
make test-e2e      # E2E tests using Kind cluster (requires Kind installed)

# Run a single test (Ginkgo)
go test ./internal/controller/... -v -run "TestControllers/YourTestName"

# Build
make build         # Build manager binary → bin/manager
make docker-build  # Build controller image
make docker-build IMG=myrepo/image:tag DOCKERFILE=Dockerfile.agent  # Agent image

# Cluster deployment
make install       # Install CRDs into current cluster context
make deploy IMG=<image>  # Deploy controller
make undeploy      # Remove controller
make uninstall     # Remove CRDs
```

## Architecture

### CRD: `PodTracker` (`api/v1/podtracker_types.go`)

- API Group: `networking.learntosolveit.com/v1`
- Cluster-scoped resource
- `spec.namespace`: if set, tracks only that namespace; empty = all namespaces
- `status.podIPs`: global flat list of all `PodInfo` entries (written by controller)
- `status.nodeInfo`: per-node list with nested `PodInfo` and `NodeResources` (written by agents)

### Controller (`internal/controller/podtracker_controller.go`)

- Watches `PodTracker` and `Pod` resources
- Pod events trigger reconciliation of any `PodTracker` whose namespace filter matches
- Adds a finalizer to handle clean deletion
- Reconcile loop: list pods → diff against current status → patch status if changed → requeue after 5 minutes
- Preserves existing `nodeInfo` data from agents when updating status

### Node Agent (`cmd/agent/main.go`)

- Reads `NODE_NAME` from downward API env var
- Every 60 seconds: lists pods on this node, fetches node resource capacity/allocatable, updates all `PodTracker` CRs using optimistic concurrency retry

### Config (`config/`)

- Kustomize-based overlays: `config/default/` is the primary overlay for the controller; `config/agent/` has the DaemonSet and its RBAC
- Default namespace: `kubernetes-ip-tracker-system`
- `config/samples/networking_v1_podtracker.yaml` is the example CR

### Testing

- Unit tests use Ginkgo v2 + Gomega with `envtest` (in-memory Kubernetes API)
- E2E tests spin up a Kind cluster, install CRDs, deploy the controller, and validate via `kubectl`
- `KUBEBUILDER_ASSETS` env var is set automatically by `make test` via `setup-envtest`

## Key Conventions

- After modifying any type in `api/v1/`, always run `make generate && make manifests`
- The agent image is built from `Dockerfile.agent` (Go 1.21 base); the controller uses `Dockerfile` (Go 1.23 base)
- Linting uses `.golangci.yml`; `api/*` is excluded from `lll`, `internal/*` is excluded from `dupl` and `lll`
