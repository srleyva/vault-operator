package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsulList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Consul `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Consul struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ConsulSpec   `json:"spec"`
	Status            ConsulStatus `json:"status,omitempty"`
}

type ConsulSpec struct {
	Server ClusterSpec `json:"server"`
	Client ClusterSpec `json:"client"`
}

type ConsulStatus struct {
	Nodes []string `json:"nodes"`
}

type ClusterSpec struct {
	Size int32 `json:"size"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VaultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Vault `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Vault struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              VaultSpec   `json:"spec"`
	Status            VaultStatus `json:"status,omitempty"`
}

type VaultSpec struct {
	Size int32 `json:"size"`
}

type VaultStatus struct {
	Nodes []string `json:"nodes"`
}
