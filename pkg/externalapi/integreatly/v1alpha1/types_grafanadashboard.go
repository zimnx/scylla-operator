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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrafanaDashboardSpec defines the desired state of GrafanaDashboard
type GrafanaDashboardSpec struct {
	// Json is the dashboard's JSON
	Json string `json:"json,omitempty"`
	// GzipJson the dashboard's JSON compressed with Gzip. Base64-encoded when in YAML.
	GzipJson []byte          `json:"gzipJson,omitempty"`
	Jsonnet  string          `json:"jsonnet,omitempty"`
	Plugins  []GrafanaPlugin `json:"plugins,omitempty"`
	Url      string          `json:"url,omitempty"`
	// ConfigMapRef is a reference to a ConfigMap data field containing the dashboard's JSON
	ConfigMapRef *corev1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
	// GzipConfigMapRef is a reference to a ConfigMap binaryData field containing
	// the dashboard's JSON, compressed with Gzip.
	GzipConfigMapRef *corev1.ConfigMapKeySelector      `json:"gzipConfigMapRef,omitempty"`
	Datasources      []GrafanaDashboardDatasource      `json:"datasources,omitempty"`
	CustomFolderName string                            `json:"customFolderName,omitempty"`
	GrafanaCom       *GrafanaDashboardGrafanaComSource `json:"grafanaCom,omitempty"`

	// ContentCacheDuration sets how often the operator should resync with the external source when using
	// the `grafanaCom.id` or `url` field to specify the source of the dashboard. The default value is
	// decided by the `dashboardContentCacheDuration` field in the `Grafana` resource. The default is 0 which
	// is interpreted as never refetching.
	ContentCacheDuration *metav1.Duration `json:"contentCacheDuration,omitempty"`
}

type GrafanaDashboardDatasource struct {
	InputName      string `json:"inputName"`
	DatasourceName string `json:"datasourceName"`
}

type GrafanaDashboardGrafanaComSource struct {
	Id       int  `json:"id"`
	Revision *int `json:"revision,omitempty"`
}

// GrafanaDashboardRef is used to keep a dashboard reference without having access to the dashboard
// struct itself
type GrafanaDashboardRef struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	UID        string `json:"uid"`
	Hash       string `json:"hash"`
	FolderId   *int64 `json:"folderId"`
	FolderName string `json:"folderName"`
}

type GrafanaDashboardStatus struct {
	ContentCache     []byte                 `json:"contentCache,omitempty"`
	ContentTimestamp *metav1.Time           `json:"contentTimestamp,omitempty"`
	ContentUrl       string                 `json:"contentUrl,omitempty"`
	Error            *GrafanaDashboardError `json:"error,omitempty"`
}

type GrafanaDashboardError struct {
	Code    int    `json:"code"`
	Message string `json:"error"`
	Retries int    `json:"retries,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GrafanaDashboard is the Schema for the grafanadashboards API
type GrafanaDashboard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrafanaDashboardSpec   `json:"spec,omitempty"`
	Status GrafanaDashboardStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// GrafanaDashboardList contains a list of GrafanaDashboard
type GrafanaDashboardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrafanaDashboard `json:"items"`
}
