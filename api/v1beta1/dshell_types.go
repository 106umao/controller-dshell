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

	// Command 客户端设置 .spec.command 的值向 k8s 集群上的 controller 下发 shell 命令
	Command string `json:"command"`

	// todo 具有单位的变量，把单位体现在变量名中
	// Timeout shell 命令执行的超时时间，默认为 0，单位毫秒
	Timeout int64 `json:"timeout,omitempty"`
}

// DShellStatus defines the observed state of DShell
type DShellStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Command 当前 CR 执行的 shell 命令
	Command string `json:"command,omitempty"`

	// NodesResults 集群中所有的 controller pod 执行 shell 命令的结果列表
	NodesResults []ExecResult `json:"nodesResults,omitempty"`
}

type ExecResult struct {
	// PodName controller 的宿主 pod 名称
	PodName string `json:"podName,omitempty"`

	// podIp controller 的宿主 pod IP
	PodIp string `json:"podIp,omitempty"`

	// StartTime 命令执行的开始时间
	StartTime metav1.Time `json:"startTime,omitempty"`

	// EndTime 命令执行的结束时间
	EndTime metav1.Time `json:"endTime,omitempty"`

	// Stdout shell 命令执行的标准输出流内容
	Stdout string `json:"stdout,omitempty"`

	// Stderr shell 命令执行的标准错误流内容
	Stderr string `json:"stderr,omitempty"`
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
