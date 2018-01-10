//go:generate ../../../../vendor/k8s.io/code-generator/generate-groups.sh all github.com/bitnami-labs/helm-crd/pkg/client github.com/bitnami-labs/helm-crd/pkg/apis helm.bitnami.com:v1
// +k8s:deepcopy-gen=package,register

// +groupName=helm.bitnami.com
package v1
