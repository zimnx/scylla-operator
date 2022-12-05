/*
Copyright 2021.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GrafanaPermissionItem struct {
	PermissionTargetType string `json:"permissionTargetType"`
	PermissionTarget     string `json:"permissionTarget"`
	PermissionLevel      int    `json:"permissionLevel"`
}

type GrafanaFolderSpec struct {
	// FolderName is the display-name of the folder and must match CustomFolderName of any GrafanaDashboard you want to put in
	FolderName string `json:"title"`

	// FolderPermissions shall contain the _complete_ permissions for the folder.
	// Any permission not listed here, will be removed from the folder.
	FolderPermissions []GrafanaPermissionItem `json:"permissions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GrafanaFolder is the Schema for the grafana folders and folderpermissions API
type GrafanaFolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GrafanaFolderSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GrafanaFolderList contains a list of GrafanaFolder
type GrafanaFolderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrafanaFolder `json:"items"`
}

// GrafanaFolderRef is used to keep a folder reference without having access to the folder-struct itself
type GrafanaFolderRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Hash      string `json:"hash"`
}
