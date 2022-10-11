/*
Copyright 2022.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DShellSpec defines the desired state of DShell
type DShellSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Command client update this field to distribute shell command event.
	Command string `json:"command,omitempty"`
}

// DShellStatus defines the observed state of DShell
type DShellStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// LastCommand is executed last time.
	LastCommand string `json:"lastCommand,omitempty"`

	// ExecuteHistories is distribution shell execution result histories.
	ExecuteHistories []ExecResult `json:"executeHistories,omitempty"`
}

type ExecResult struct {
	// Command shell command from CR event.
	Command string `json:"command,omitempty"`

	// ExecuteTime command execution time.
	ExecuteTime metav1.Time `json:"executeTime,omitempty"`

	// NodeExecResults command relation to result is 1 to N.
	NodeExecResults []NodeResult `json:"nodeExecResults,omitempty"`
}

type NodeResult struct {
	// Addresses is nodes ip info of k8s cluster.
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// Message is shell execution result of current nodes.
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DShell is the Schema for the dshells API
type DShell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DShellSpec   `json:"spec,omitempty"`
	Status DShellStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DShellList contains a list of DShell
type DShellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DShell `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DShell{}, &DShellList{})
}
