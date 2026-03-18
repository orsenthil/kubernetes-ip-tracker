make install
mkdir -p /Users/senthil/git/kubernetes-ip-tracker/bin
Downloading sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.2
go: downloading sigs.k8s.io/controller-tools v0.17.2
go: downloading github.com/spf13/cobra v1.8.1
go: downloading github.com/fatih/color v1.18.0
go: downloading k8s.io/api v0.32.1
go: downloading k8s.io/apimachinery v0.32.1
go: downloading gopkg.in/yaml.v3 v3.0.1
go: downloading k8s.io/apiextensions-apiserver v0.32.1
go: downloading sigs.k8s.io/yaml v1.4.0
go: downloading golang.org/x/tools v0.29.0
go: downloading gopkg.in/yaml.v2 v2.4.0
go: downloading github.com/gobuffalo/flect v1.0.3
go: downloading k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738
go: downloading github.com/mattn/go-colorable v0.1.13
go: downloading github.com/mattn/go-isatty v0.0.20
go: downloading github.com/spf13/pflag v1.0.6
go: downloading github.com/gogo/protobuf v1.3.2
go: downloading github.com/google/gofuzz v1.2.0
go: downloading k8s.io/klog/v2 v2.130.1
go: downloading sigs.k8s.io/structured-merge-diff/v4 v4.4.2
go: downloading gopkg.in/inf.v0 v0.9.1
go: downloading sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3
go: downloading golang.org/x/sys v0.29.0
go: downloading github.com/fxamacker/cbor/v2 v2.7.0
go: downloading golang.org/x/net v0.34.0
go: downloading github.com/go-logr/logr v1.4.2
go: downloading github.com/json-iterator/go v1.1.12
go: downloading golang.org/x/sync v0.10.0
go: downloading golang.org/x/mod v0.22.0
go: downloading github.com/x448/float16 v0.8.4
go: downloading github.com/modern-go/reflect2 v1.0.2
go: downloading github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
go: downloading golang.org/x/text v0.21.0
/Users/senthil/git/kubernetes-ip-tracker/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
Downloading sigs.k8s.io/kustomize/kustomize/v5@v5.5.0
go: downloading sigs.k8s.io/kustomize/kustomize/v5 v5.5.0
go: downloading sigs.k8s.io/kustomize/api v0.18.0
go: downloading sigs.k8s.io/kustomize/cmd/config v0.15.0
go: downloading github.com/spf13/cobra v1.8.0
go: downloading sigs.k8s.io/kustomize/kyaml v0.18.1
go: downloading golang.org/x/text v0.16.0
go: downloading github.com/sergi/go-diff v1.2.0
go: downloading github.com/blang/semver/v4 v4.0.0
go: downloading k8s.io/kube-openapi v0.0.0-20231010175941-2dd684a91f00
go: downloading github.com/go-errors/errors v1.4.2
go: downloading github.com/xlab/treeprint v1.2.0
go: downloading github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00
go: downloading google.golang.org/protobuf v1.33.0
go: downloading github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
go: downloading github.com/golang/protobuf v1.5.3
go: downloading github.com/go-openapi/swag v0.22.4
go: downloading github.com/go-openapi/jsonpointer v0.19.6
/Users/senthil/git/kubernetes-ip-tracker/bin/kustomize build config/crd | kubectl apply -f -
customresourcedefinition.apiextensions.k8s.io/podtrackers.networking.learntosolveit.com created


make deploy IMG=docker.io/skumaran/kubernetes-ip-tracker:v0.1.0
/Users/senthil/git/kubernetes-ip-tracker/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
cd config/manager && /Users/senthil/git/kubernetes-ip-tracker/bin/kustomize edit set image controller=docker.io/skumaran/kubernetes-ip-tracker:v0.1.0
/Users/senthil/git/kubernetes-ip-tracker/bin/kustomize build config/default | kubectl apply -f -
namespace/kubernetes-ip-tracker-system created
customresourcedefinition.apiextensions.k8s.io/podtrackers.networking.learntosolveit.com unchanged
serviceaccount/kubernetes-ip-tracker-controller-manager created
role.rbac.authorization.k8s.io/kubernetes-ip-tracker-leader-election-role created
clusterrole.rbac.authorization.k8s.io/kubernetes-ip-tracker-manager-role created
clusterrole.rbac.authorization.k8s.io/kubernetes-ip-tracker-metrics-auth-role created
clusterrole.rbac.authorization.k8s.io/kubernetes-ip-tracker-metrics-reader created
clusterrole.rbac.authorization.k8s.io/kubernetes-ip-tracker-podtracker-admin-role created
clusterrole.rbac.authorization.k8s.io/kubernetes-ip-tracker-podtracker-editor-role created
clusterrole.rbac.authorization.k8s.io/kubernetes-ip-tracker-podtracker-viewer-role created
rolebinding.rbac.authorization.k8s.io/kubernetes-ip-tracker-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/kubernetes-ip-tracker-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/kubernetes-ip-tracker-metrics-auth-rolebinding created
service/kubernetes-ip-tracker-controller-manager-metrics-service created
deployment.apps/kubernetes-ip-tracker-controller-manager created
