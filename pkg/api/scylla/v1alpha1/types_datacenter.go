// Copyright (c) 2022 ScyllaDB.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScyllaDatacenterSpec defines the desired state of Datacenter.
type ScyllaDatacenterSpec struct {
	// image holds a reference to the Scylla container image.
	// +optional
	Image string `json:"image"`

	// managerAgentImageholds a reference to the Scylla Manager Agent container image.
	// +optional
	AgentImage string `json:"agentImage"`

	// alternator designates this cluster an Alternator cluster.
	// +optional
	Alternator *AlternatorSpec `json:"alternator,omitempty"`

	// cpuset determines if the cluster will use cpu-pinning for max performance.
	// +optional
	CpuSet *bool `json:"cpuset,omitempty"`

	// developerMode determines if the cluster runs in developer-mode.
	// +kubebuilder:default:=false
	// +optional
	EnableDeveloperMode *bool `json:"enableDeveloperMode,omitempty"`

	// removeOrphanedPVs allows the controller to delete PVs bound to nodes that no longer exist and proceed by recreating the PV on some other node.
	// +optional
	// +kubebuilder:default:=true
	RemoveOrphanedPVs *bool `json:"removeOrphanedPVs,omitempty"`

	// genericUpgrade allows to configure behavior of generic upgrade logic.
	// +optional
	GenericUpgrade *GenericUpgradeSpec `json:"genericUpgrade,omitempty"`

	// datacenter holds a specification of a Scylla datacenter.
	Datacenter DatacenterSpec `json:"datacenter"`

	// sysctls holds the sysctl properties to be applied during initialization given as a list of key=value pairs.
	// Example: fs.aio-max-nr=232323
	// +optional
	Sysctls []string `json:"sysctls,omitempty"`

	// scyllaArgs will be appended to the Scylla binary during startup.
	// +optional
	UnsupportedScyllaArgsOverrides []string `json:"unsupportedScyllaArgsOverrides,omitempty"`

	// network holds the networking config.
	// +optional
	Network Network `json:"network,omitempty"`

	// repairs specify repair tasks in Scylla Manager.
	// When Scylla Manager is not installed, these will be ignored.
	// +optional
	Repairs []RepairTaskSpec `json:"repairs,omitempty"`

	// backups specifies backup tasks in Scylla Manager.
	// When Scylla Manager is not installed, these will be ignored.
	// +optional
	Backups []BackupTaskSpec `json:"backups,omitempty"`

	// forceRedeploymentReason can be used to force a rolling update of all racks by providing a unique string.
	// +optional
	ForceRedeploymentReason string `json:"forceRedeploymentReason,omitempty"`

	// imagePullSecrets is an optional list of references to secrets in the same namespace
	// used for pulling Scylla and Agent images.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// DatacenterSpec is the desired state for a Scylla Datacenter.
type DatacenterSpec struct {
	// name is the name of the scylla datacenter. Used in the cassandra-rackdc.properties file.
	Name string `json:"name"`

	// racks specify the racks in the datacenter.
	Racks []RackSpec `json:"racks"`
}

// GenericUpgradeFailureStrategy allows to specify how upgrade logic should handle failures.
type GenericUpgradeFailureStrategy string

const (
	// GenericUpgradeFailureStrategyRetry infinitely retries until node becomes ready.
	GenericUpgradeFailureStrategyRetry GenericUpgradeFailureStrategy = "Retry"
)

// GenericUpgradeSpec hold generic upgrade procedure parameters.
type GenericUpgradeSpec struct {
	// failureStrategy specifies which logic is executed when upgrade failure happens.
	// Currently only Retry is supported.
	// +kubebuilder:default:="Retry"
	// +optional
	FailureStrategy GenericUpgradeFailureStrategy `json:"failureStrategy,omitempty"`
}

type SchedulerTaskSpec struct {
	// name is a unique name of a task.
	Name string `json:"name"`

	// startDate specifies the task start date expressed in the RFC3339 format or now[+duration],
	// e.g. now+3d2h10m, valid units are d, h, m, s.
	// +kubebuilder:default:="now"
	// +optional
	StartDate string `json:"startDate,omitempty"`

	// interval represents a task schedule interval e.g. 3d2h10m, valid units are d, h, m, s.
	// +optional
	// +kubebuilder:default:="0"
	Interval string `json:"interval,omitempty"`

	// numRetries indicates how many times a scheduled task will be retried before failing.
	// +kubebuilder:default:=3
	// +optional
	NumRetries *int64 `json:"numRetries,omitempty"`
}

type RepairTaskSpec struct {
	SchedulerTaskSpec `json:",inline"`

	// dc is a list of datacenter glob patterns, e.g. 'dc1', '!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`

	// failFast indicates if a repair should be stopped on first error.
	// +optional
	FailFast bool `json:"failFast,omitempty" mapstructure:"fail_fast,omitempty"`

	// intensity indicates how many token ranges (per shard) to repair in a single Scylla repair job. By default this is 1.
	// If you set it to 0 the number of token ranges is adjusted to the maximum supported by node (see max_repair_ranges_in_parallel in Scylla logs).
	// Valid values are 0 and integers >= 1. Higher values will result in increased cluster load and slightly faster repairs.
	// Changing the intensity impacts repair granularity if you need to resume it, the higher the value the more work on resume.
	// For Scylla clusters that *do not support row-level repair*, intensity can be a decimal between (0,1).
	// In that case it specifies percent of shards that can be repaired in parallel on a repair master node.
	// For Scylla clusters that are row-level repair enabled, setting intensity below 1 has the same effect as setting intensity 1.
	// +kubebuilder:default:="1"
	// +optional
	Intensity string `json:"intensity,omitempty" mapstructure:"intensity,omitempty"`

	// parallel is the maximum number of Scylla repair jobs that can run at the same time (on different token ranges and replicas).
	// Each node can take part in at most one repair at any given moment. By default the maximum possible parallelism is used.
	// The effective parallelism depends on a keyspace replication factor (RF) and the number of nodes.
	// The formula to calculate it is as follows: number of nodes / RF, ex. for 6 node cluster with RF=3 the maximum parallelism is 2.
	// +kubebuilder:default:=0
	// +optional
	Parallel int64 `json:"parallel,omitempty" mapstructure:"parallel,omitempty"`

	// keyspace is a list of keyspace/tables glob patterns, e.g. 'keyspace,!keyspace.table_prefix_*'
	// used to include or exclude keyspaces from repair.
	Keyspace []string `json:"keyspace,omitempty" mapstructure:"keyspace,omitempty"`

	// smallTableThreshold enable small table optimization for tables of size lower than given threshold.
	// Supported units [B, MiB, GiB, TiB].
	// +kubebuilder:default:="1GiB"
	// +optional
	SmallTableThreshold string `json:"smallTableThreshold,omitempty" mapstructure:"small_table_threshold,omitempty"`

	// host specifies a host to repair. If empty, all hosts are repaired.
	Host *string `json:"host,omitempty" mapstructure:"host,omitempty"`
}

type BackupTaskSpec struct {
	SchedulerTaskSpec `json:",inline"`

	// dc is a list of datacenter glob patterns, e.g. 'dc1,!otherdc*' used to specify the DCs
	// to include or exclude from backup.
	// +optional
	DC []string `json:"dc,omitempty" mapstructure:"dc,omitempty"`

	// keyspace is a list of keyspace/tables glob patterns,
	// e.g. 'keyspace,!keyspace.table_prefix_*' used to include or exclude keyspaces from repair.
	// +optional
	Keyspace []string `json:"keyspace,omitempty" mapstructure:"keyspace,omitempty"`

	// location is a list of backup locations in the format [<dc>:]<provider>:<name> ex. s3:my-bucket.
	// The <dc>: part is optional and is only needed when different datacenters are being used to upload data
	// to different locations. <name> must be an alphanumeric string and may contain a dash and or a dot,
	// but other characters are forbidden.
	// The only supported storage <provider> at the moment are s3 and gcs.
	Location []string `json:"location" mapstructure:"location,omitempty"`

	// rateLimit is a list of megabytes (MiB) per second rate limits expressed in the format [<dc>:]<limit>.
	// The <dc>: part is optional and only needed when different datacenters need different upload limits.
	// Set to 0 for no limit (default 100).
	// +optional
	RateLimit []string `json:"rateLimit,omitempty" mapstructure:"rate_limit,omitempty"`

	// retention is the number of backups which are to be stored.
	// +kubebuilder:default:=3
	// +optional
	Retention int64 `json:"retention,omitempty" mapstructure:"retention,omitempty"`

	// snapshotParallel is a list of snapshot parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set, the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	// +optional
	SnapshotParallel []string `json:"snapshotParallel,omitempty" mapstructure:"snapshot_parallel,omitempty"`

	// uploadParallel is a list of upload parallelism limits in the format [<dc>:]<limit>.
	// The <dc>: part is optional and allows for specifying different limits in selected datacenters.
	// If The <dc>: part is not set the limit is global (e.g. 'dc1:2,5') the runs are parallel in n nodes (2 in dc1)
	// and n nodes in all the other datacenters.
	// +optional
	UploadParallel []string `json:"uploadParallel,omitempty" mapstructure:"upload_parallel,omitempty"`
}

type Network struct {
	// hostNetworking determines if scylla uses the host's network namespace. Setting this option
	// avoids going through Kubernetes SDN and exposes scylla on node's IP.
	HostNetworking bool `json:"hostNetworking,omitempty"`

	// dnsPolicy defines how a pod's DNS will be configured.
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
}

func (s Network) GetDNSPolicy() corev1.DNSPolicy {
	if len(s.DNSPolicy) != 0 {
		return s.DNSPolicy
	}

	return corev1.DNSClusterFirstWithHostNet
}

// RackSpec is the desired state for a Scylla Rack.
type RackSpec struct {
	// name is the name of the Scylla Rack. Used in the cassandra-rackdc.properties file.
	Name string `json:"name"`

	// members is the number of Scylla instances in this rack.
	// +kubebuilder:default:=1
	// +optional
	Members *int32 `json:"members"`

	// storage describes the underlying storage that Scylla will consume.
	Storage StorageSpec `json:"storage"`

	// placement describes restrictions for the nodes Scylla is scheduled on.
	// +optional
	Placement *PlacementSpec `json:"placement,omitempty"`

	// scyllaContainer describes properties of Scylla container.
	ScyllaContainer ScyllaContainerSpec `json:"scyllaContainer"`

	// agentContainer describes properties of Scylla Manager Agent container.
	AgentContainer AgentContainerSpec `json:"agentContainer"`
}

// ContainerSpec is the desired state of the container.
type ContainerSpec struct {
	// resources requirements for the container.
	Resources corev1.ResourceRequirements `json:"resources"`

	// Volumes to be added to the container.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts to be added to the container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

type AgentContainerSpec struct {
	ContainerSpec `json:",inline"`

	// customConfigSecretRef is a reference to Secret holding custom Scylla Manager Agent configuration.
	// +optional
	CustomConfigSecretRef *corev1.LocalObjectReference `json:"customConfigSecretRef,omitempty"`
}

type ScyllaContainerSpec struct {
	ContainerSpec `json:",inline"`

	// customConfigMapRef is a reference to ConfigMap holding custom Scylla configuration.
	// +optional
	CustomConfigMapRef *corev1.LocalObjectReference `json:"customConfigMapRef,omitempty"`

	// customConfigRaw is a raw custom Scylla configuration.
	// +optional
	CustomConfigRaw string `json:"customConfigRaw,omitempty"`
}

type PlacementSpec struct {
	// nodeAffinity describes node affinity scheduling rules for the pod.
	// +optional
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`

	// podAffinity describes pod affinity scheduling rules.
	// +optional
	PodAffinity *corev1.PodAffinity `json:"podAffinity,omitempty"`

	// podAntiAffinity describes pod anti-affinity scheduling rules.
	// +optional
	PodAntiAffinity *corev1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`

	// tolerations allow the pod to tolerate any taint that matches the triple <key,value,effect>
	// using the matching operator.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

type StorageSpec struct {
	// capacity describes the requested size of each persistent volume.
	Capacity string `json:"capacity"`

	// storageClassName is the name of a storageClass to request.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

type AlternatorSpec struct {
	// port is the port number used to bind the Alternator API.
	Port int32 `json:"port,omitempty"`

	// writeIsolation indicates the isolation level.
	WriteIsolation string `json:"writeIsolation,omitempty"`
}

func (a *AlternatorSpec) Enabled() bool {
	return a != nil && a.Port > 0
}

type RepairTaskStatus struct {
	RepairTaskSpec `json:",inline" mapstructure:",squash"`

	// id is the identification number of the repair task.
	ID string `json:"id"`

	// error holds the repair task error, if any.
	Error string `json:"error"`
}

type BackupTaskStatus struct {
	BackupTaskSpec `json:",inline"`

	// id is the identification number of the backup task.
	ID string `json:"id"`

	// error holds the backup task error, if any.
	Error string `json:"error"`
}

// ScyllaDatacenterStatus defines the observed state of ScyllaDatacenter.
type ScyllaDatacenterStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDatacenter. It corresponds to the
	// ScyllaDatacenter's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// racks reflect status of cluster racks.
	Racks map[string]RackStatus `json:"racks,omitempty"`

	// managerId contains ID under which cluster was registered in Scylla Manager.
	ManagerID *string `json:"managerId,omitempty"`

	// repairs reflects status of repair tasks.
	Repairs []RepairTaskStatus `json:"repairs,omitempty"`

	// backups reflects status of backup tasks.
	Backups []BackupTaskStatus `json:"backups,omitempty"`

	// upgrade reflects state of ongoing upgrade procedure.
	Upgrade *UpgradeStatus `json:"upgrade,omitempty"`
}

// UpgradeStatus contains the internal state of an ongoing upgrade procedure.
// Do not rely on these internal values externally. They are meant for keeping an internal state
// and their values are subject to change within the limits of API compatibility.
type UpgradeStatus struct {
	// state reflects current upgrade state.
	State string `json:"state"`

	// currentNode node under upgrade.
	// DEPRECATED.
	CurrentNode string `json:"currentNode,omitempty"`

	// currentRack rack under upgrade.
	// DEPRECATED.
	CurrentRack string `json:"currentRack,omitempty"`

	// fromVersion reflects from which version ScyllaDatacenter is being upgraded.
	FromVersion string `json:"fromVersion"`

	// toVersion reflects to which version ScyllaDatacenter is being upgraded.
	ToVersion string `json:"toVersion"`

	// systemSnapshotTag is the snapshot tag of system keyspaces.
	SystemSnapshotTag string `json:"systemSnapshotTag,omitempty"`

	// dataSnapshotTag is the snapshot tag of data keyspaces.
	DataSnapshotTag string `json:"dataSnapshotTag,omitempty"`
}

// RackStatus is the status of a Scylla Rack
type RackStatus struct {
	// Image is the current image of Scylla in use.
	Image string `json:"image"`

	// members is the current number of members requested in the specific Rack
	Members *int32 `json:"members,omitempty"`

	// readyMembers is the number of ready members in the specific Rack
	ReadyMembers *int32 `json:"readyMembers,omitempty"`

	// updatedMembers is the number of members matching the current spec.
	// +optional
	UpdatedMembers *int32 `json:"updatedMembers,omitempty"`

	// stale indicates if the current rack status is collected for a previous generation.
	// stale should eventually become false when the appropriate controller writes a fresh status.
	// +optional
	Stale *bool `json:"stale,omitempty"`

	// conditions are the latest available observations of a rack's state.
	Conditions []RackCondition `json:"conditions,omitempty"`

	// replaceAddressFirstBoot holds addresses which should be replaced by new nodes.
	ReplaceAddressFirstBoot map[string]string `json:"replaceAddressFirstBoot,omitempty"`
}

// RackCondition is an observation about the state of a rack.
type RackCondition struct {
	// type holds the condition type.
	Type RackConditionType `json:"type"`

	// status represent condition status.
	Status corev1.ConditionStatus `json:"status"`
}

type RackConditionType string

const (
	RackConditionTypeMemberLeaving         RackConditionType = "MemberLeaving"
	RackConditionTypeUpgrading             RackConditionType = "RackUpgrading"
	RackConditionTypeMemberReplacing       RackConditionType = "MemberReplacing"
	RackConditionTypeMemberDecommissioning RackConditionType = "MemberDecommissioning"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaDatacenter defines a single datacenter of Scylla cluster.
type ScyllaDatacenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of this scylla cluster.
	Spec ScyllaDatacenterSpec `json:"spec,omitempty"`

	// status is the current status of this scylla cluster.
	Status ScyllaDatacenterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScyllaDatacenterList holds a list of ScyllaDatacenters.
type ScyllaDatacenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDatacenter `json:"items"`
}
