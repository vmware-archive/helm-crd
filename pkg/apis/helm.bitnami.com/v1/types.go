package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:noStatus

// HelmRelease describes a Helm chart release.
type HelmRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HelmReleaseSpec `json:"spec"`
}

// HelmReleaseSpec is the spec for a HelmRelease resource.
type HelmReleaseSpec struct {
	// RepoURL is the URL of the repository. Defaults to stable repo.
	RepoURL string `json:"repoUrl,omitempty"`
	// ChartName is the name of the chart within the repo
	ChartName string `json:"chartName,omitempty"`
	// Version is the chart version
	Version string `json:"version,omitempty"`
	// Auth is the authentication
	Auth HelmReleaseAuth `json:"auth,omitempty"`
	// Values is a string containing (unparsed) YAML values
	Values string `json:"values,omitempty"`
}

type HelmReleaseAuth struct {
	// Header is header based Authorization
	Header *HelmReleaseAuthHeader `json:"header,omitempty"`
}

type HelmReleaseAuthHeader struct {
	// Selects a key of a secret in the pod's namespace
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HelmReleaseList is a list of HelmRelease resources
type HelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []HelmRelease `json:"items"`
}
