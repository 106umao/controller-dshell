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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DShellSpec defines the desired state of DShell
type DShellSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Command the http client sets the value of .spec.command to issue shell commands to the controller on the k8s cluster
	Command string `json:"command"`

	// TimeoutMs the timeout for shell command execution, its default is 0, in milliseconds
	TimeoutMs int64 `json:"timeout,omitempty"`
}

// DShellStatus defines the observed state of DShell
type DShellStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Command shell command executed by the current CR
	Command string `json:"command,omitempty"`

	// NodesResults list of results of shell commands executed by all controller pods in the cluster
	NodesResults []ExecResult `json:"nodesResults,omitempty"`
}

type ExecResult struct {
	// PodName the host pod name of the controller
	PodName string `json:"podName,omitempty"`

	// podIp the host pod IP of the controller
	PodIp string `json:"podIp,omitempty"`

	// StartTime of command execution
	StartTime metav1.Time `json:"startTime,omitempty"`

	// EndTime of command execution
	EndTime metav1.Time `json:"endTime,omitempty"`

	// Stdout the standard output stream of the execution shell
	Stdout string `json:"stdout,omitempty"`

	// Stderr the standard error output stream of the execution shell
	Stderr string `json:"stderr,omitempty"`

	// CtrlErr error message of dshell controller
	CtrlErr string `json:"ctrlErr,omitempty"`
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
