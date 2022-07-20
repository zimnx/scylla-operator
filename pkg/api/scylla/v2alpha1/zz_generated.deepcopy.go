//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by deepcopy-gen. DO NOT EDIT.

package v2alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlternatorOptions) DeepCopyInto(out *AlternatorOptions) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlternatorOptions.
func (in *AlternatorOptions) DeepCopy() *AlternatorOptions {
	if in == nil {
		return nil
	}
	out := new(AlternatorOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CQLExposeOptions) DeepCopyInto(out *CQLExposeOptions) {
	*out = *in
	if in.Ingress != nil {
		in, out := &in.Ingress, &out.Ingress
		*out = new(IngressOptions)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CQLExposeOptions.
func (in *CQLExposeOptions) DeepCopy() *CQLExposeOptions {
	if in == nil {
		return nil
	}
	out := new(CQLExposeOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Datacenter) DeepCopyInto(out *Datacenter) {
	*out = *in
	if in.DNSDomains != nil {
		in, out := &in.DNSDomains, &out.DNSDomains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.RemoteKubeClusterConfigRef != nil {
		in, out := &in.RemoteKubeClusterConfigRef, &out.RemoteKubeClusterConfigRef
		*out = new(RemoteKubeClusterConfigRef)
		**out = **in
	}
	if in.ExposeOptions != nil {
		in, out := &in.ExposeOptions, &out.ExposeOptions
		*out = new(ExposeOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.NodesPerRack != nil {
		in, out := &in.NodesPerRack, &out.NodesPerRack
		*out = new(int32)
		**out = **in
	}
	if in.Scylla != nil {
		in, out := &in.Scylla, &out.Scylla
		*out = new(ScyllaOverrides)
		(*in).DeepCopyInto(*out)
	}
	if in.ScyllaManagerAgent != nil {
		in, out := &in.ScyllaManagerAgent, &out.ScyllaManagerAgent
		*out = new(ScyllaManagerAgentOverrides)
		(*in).DeepCopyInto(*out)
	}
	if in.Racks != nil {
		in, out := &in.Racks, &out.Racks
		*out = make([]RackSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Placement != nil {
		in, out := &in.Placement, &out.Placement
		*out = new(Placement)
		(*in).DeepCopyInto(*out)
	}
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(Metadata)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Datacenter.
func (in *Datacenter) DeepCopy() *Datacenter {
	if in == nil {
		return nil
	}
	out := new(Datacenter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DatacenterStatus) DeepCopyInto(out *DatacenterStatus) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = new(int32)
		**out = **in
	}
	if in.CurrentNodes != nil {
		in, out := &in.CurrentNodes, &out.CurrentNodes
		*out = new(int32)
		**out = **in
	}
	if in.UpdatedNodes != nil {
		in, out := &in.UpdatedNodes, &out.UpdatedNodes
		*out = new(int32)
		**out = **in
	}
	if in.ReadyNodes != nil {
		in, out := &in.ReadyNodes, &out.ReadyNodes
		*out = new(int32)
		**out = **in
	}
	if in.Stale != nil {
		in, out := &in.Stale, &out.Stale
		*out = new(bool)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Racks != nil {
		in, out := &in.Racks, &out.Racks
		*out = make([]RackStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Upgrade != nil {
		in, out := &in.Upgrade, &out.Upgrade
		*out = new(UpgradeStatus)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DatacenterStatus.
func (in *DatacenterStatus) DeepCopy() *DatacenterStatus {
	if in == nil {
		return nil
	}
	out := new(DatacenterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExposeOptions) DeepCopyInto(out *ExposeOptions) {
	*out = *in
	if in.CQL != nil {
		in, out := &in.CQL, &out.CQL
		*out = new(CQLExposeOptions)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExposeOptions.
func (in *ExposeOptions) DeepCopy() *ExposeOptions {
	if in == nil {
		return nil
	}
	out := new(ExposeOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressOptions) DeepCopyInto(out *IngressOptions) {
	*out = *in
	if in.Disabled != nil {
		in, out := &in.Disabled, &out.Disabled
		*out = new(bool)
		**out = **in
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressOptions.
func (in *IngressOptions) DeepCopy() *IngressOptions {
	if in == nil {
		return nil
	}
	out := new(IngressOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Metadata) DeepCopyInto(out *Metadata) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Metadata.
func (in *Metadata) DeepCopy() *Metadata {
	if in == nil {
		return nil
	}
	out := new(Metadata)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Network) DeepCopyInto(out *Network) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Network.
func (in *Network) DeepCopy() *Network {
	if in == nil {
		return nil
	}
	out := new(Network)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Placement) DeepCopyInto(out *Placement) {
	*out = *in
	if in.NodeAffinity != nil {
		in, out := &in.NodeAffinity, &out.NodeAffinity
		*out = new(corev1.NodeAffinity)
		(*in).DeepCopyInto(*out)
	}
	if in.PodAffinity != nil {
		in, out := &in.PodAffinity, &out.PodAffinity
		*out = new(corev1.PodAffinity)
		(*in).DeepCopyInto(*out)
	}
	if in.PodAntiAffinity != nil {
		in, out := &in.PodAntiAffinity, &out.PodAntiAffinity
		*out = new(corev1.PodAntiAffinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Placement.
func (in *Placement) DeepCopy() *Placement {
	if in == nil {
		return nil
	}
	out := new(Placement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RackSpec) DeepCopyInto(out *RackSpec) {
	*out = *in
	if in.Placement != nil {
		in, out := &in.Placement, &out.Placement
		*out = new(Placement)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RackSpec.
func (in *RackSpec) DeepCopy() *RackSpec {
	if in == nil {
		return nil
	}
	out := new(RackSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RackStatus) DeepCopyInto(out *RackStatus) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = new(int32)
		**out = **in
	}
	if in.CurrentNodes != nil {
		in, out := &in.CurrentNodes, &out.CurrentNodes
		*out = new(int32)
		**out = **in
	}
	if in.UpdatedNodes != nil {
		in, out := &in.UpdatedNodes, &out.UpdatedNodes
		*out = new(int32)
		**out = **in
	}
	if in.ReadyNodes != nil {
		in, out := &in.ReadyNodes, &out.ReadyNodes
		*out = new(int32)
		**out = **in
	}
	if in.Stale != nil {
		in, out := &in.Stale, &out.Stale
		*out = new(bool)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ReplaceAddressFirstBoot != nil {
		in, out := &in.ReplaceAddressFirstBoot, &out.ReplaceAddressFirstBoot
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RackStatus.
func (in *RackStatus) DeepCopy() *RackStatus {
	if in == nil {
		return nil
	}
	out := new(RackStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RemoteKubeClusterConfigRef) DeepCopyInto(out *RemoteKubeClusterConfigRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RemoteKubeClusterConfigRef.
func (in *RemoteKubeClusterConfigRef) DeepCopy() *RemoteKubeClusterConfigRef {
	if in == nil {
		return nil
	}
	out := new(RemoteKubeClusterConfigRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Scylla) DeepCopyInto(out *Scylla) {
	*out = *in
	if in.AlternatorOptions != nil {
		in, out := &in.AlternatorOptions, &out.AlternatorOptions
		*out = new(AlternatorOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.UnsupportedScyllaArgs != nil {
		in, out := &in.UnsupportedScyllaArgs, &out.UnsupportedScyllaArgs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EnableDeveloperMode != nil {
		in, out := &in.EnableDeveloperMode, &out.EnableDeveloperMode
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Scylla.
func (in *Scylla) DeepCopy() *Scylla {
	if in == nil {
		return nil
	}
	out := new(Scylla)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScyllaCluster) DeepCopyInto(out *ScyllaCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScyllaCluster.
func (in *ScyllaCluster) DeepCopy() *ScyllaCluster {
	if in == nil {
		return nil
	}
	out := new(ScyllaCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScyllaCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScyllaClusterList) DeepCopyInto(out *ScyllaClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScyllaCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScyllaClusterList.
func (in *ScyllaClusterList) DeepCopy() *ScyllaClusterList {
	if in == nil {
		return nil
	}
	out := new(ScyllaClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScyllaClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScyllaClusterSpec) DeepCopyInto(out *ScyllaClusterSpec) {
	*out = *in
	in.Scylla.DeepCopyInto(&out.Scylla)
	if in.ScyllaManagerAgent != nil {
		in, out := &in.ScyllaManagerAgent, &out.ScyllaManagerAgent
		*out = new(ScyllaManagerAgent)
		**out = **in
	}
	if in.Datacenters != nil {
		in, out := &in.Datacenters, &out.Datacenters
		*out = make([]Datacenter, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.Network != nil {
		in, out := &in.Network, &out.Network
		*out = new(Network)
		**out = **in
	}
	if in.Metadata != nil {
		in, out := &in.Metadata, &out.Metadata
		*out = new(Metadata)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScyllaClusterSpec.
func (in *ScyllaClusterSpec) DeepCopy() *ScyllaClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ScyllaClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScyllaClusterStatus) DeepCopyInto(out *ScyllaClusterStatus) {
	*out = *in
	if in.ObservedGeneration != nil {
		in, out := &in.ObservedGeneration, &out.ObservedGeneration
		*out = new(int64)
		**out = **in
	}
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = new(int32)
		**out = **in
	}
	if in.CurrentNodes != nil {
		in, out := &in.CurrentNodes, &out.CurrentNodes
		*out = new(int32)
		**out = **in
	}
	if in.UpdatedNodes != nil {
		in, out := &in.UpdatedNodes, &out.UpdatedNodes
		*out = new(int32)
		**out = **in
	}
	if in.ReadyNodes != nil {
		in, out := &in.ReadyNodes, &out.ReadyNodes
		*out = new(int32)
		**out = **in
	}
	if in.Datacenters != nil {
		in, out := &in.Datacenters, &out.Datacenters
		*out = make([]DatacenterStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScyllaClusterStatus.
func (in *ScyllaClusterStatus) DeepCopy() *ScyllaClusterStatus {
	if in == nil {
		return nil
	}
	out := new(ScyllaClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScyllaManagerAgent) DeepCopyInto(out *ScyllaManagerAgent) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScyllaManagerAgent.
func (in *ScyllaManagerAgent) DeepCopy() *ScyllaManagerAgent {
	if in == nil {
		return nil
	}
	out := new(ScyllaManagerAgent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScyllaManagerAgentOverrides) DeepCopyInto(out *ScyllaManagerAgentOverrides) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(corev1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.CustomConfigSecretRef != nil {
		in, out := &in.CustomConfigSecretRef, &out.CustomConfigSecretRef
		*out = new(corev1.LocalObjectReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScyllaManagerAgentOverrides.
func (in *ScyllaManagerAgentOverrides) DeepCopy() *ScyllaManagerAgentOverrides {
	if in == nil {
		return nil
	}
	out := new(ScyllaManagerAgentOverrides)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScyllaOverrides) DeepCopyInto(out *ScyllaOverrides) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(corev1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.Storage != nil {
		in, out := &in.Storage, &out.Storage
		*out = new(Storage)
		(*in).DeepCopyInto(*out)
	}
	if in.CustomConfigMapRef != nil {
		in, out := &in.CustomConfigMapRef, &out.CustomConfigMapRef
		*out = new(corev1.LocalObjectReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScyllaOverrides.
func (in *ScyllaOverrides) DeepCopy() *ScyllaOverrides {
	if in == nil {
		return nil
	}
	out := new(ScyllaOverrides)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Storage) DeepCopyInto(out *Storage) {
	*out = *in
	in.Resources.DeepCopyInto(&out.Resources)
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Storage.
func (in *Storage) DeepCopy() *Storage {
	if in == nil {
		return nil
	}
	out := new(Storage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpgradeStatus) DeepCopyInto(out *UpgradeStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpgradeStatus.
func (in *UpgradeStatus) DeepCopy() *UpgradeStatus {
	if in == nil {
		return nil
	}
	out := new(UpgradeStatus)
	in.DeepCopyInto(out)
	return out
}
