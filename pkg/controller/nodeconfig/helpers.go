// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/controller/daemon/util"
	pluginhelper "github.com/scylladb/scylla-operator/pkg/thirdparty/k8s.io/kubernetes/pkg/scheduler/framework/plugins/helper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
)

func nodeIsTargetedByDaemonSet(node *corev1.Node, ds *appsv1.DaemonSet) bool {
	shouldRun, shouldContinueRunning := nodeShouldRunDaemonPod(node, ds)
	return shouldRun || shouldContinueRunning
}

// nodeShouldRunDaemonPod checks a set of preconditions against a (node,daemonset) and returns a
// summary. Returned booleans are:
// * shouldRun:
//     Returns true when a daemonset should run on the node if a daemonset pod is not already
//     running on that node.
// * shouldContinueRunning:
//     Returns true when a daemonset should continue running on a node if a daemonset pod is already
//     running on that node.
func nodeShouldRunDaemonPod(node *corev1.Node, ds *appsv1.DaemonSet) (bool, bool) {
	pod := newDaemonSetPod(ds, node.Name)

	// If the daemon set specifies a node name, check that it matches with node.Name.
	if !(ds.Spec.Template.Spec.NodeName == "" || ds.Spec.Template.Spec.NodeName == node.Name) {
		return false, false
	}

	fitsNodeName, fitsNodeAffinity, fitsTaints := canPodRunOnNode(pod, node)
	if !fitsNodeName || !fitsNodeAffinity {
		return false, false
	}

	if !fitsTaints {
		// Scheduled daemon pods should continue running if they tolerate NoExecute taint.
		_, hasUntoleratedTaint := corev1helpers.FindMatchingUntoleratedTaint(node.Spec.Taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
			return t.Effect == corev1.TaintEffectNoExecute
		})
		return false, !hasUntoleratedTaint
	}

	return true, true
}

// Predicates checks if a pod can run on a node.
func canPodRunOnNode(pod *corev1.Pod, node *corev1.Node) (fitsNodeName, fitsNodeAffinity, fitsTaints bool) {
	fitsNodeName = len(pod.Spec.NodeName) == 0 || pod.Spec.NodeName == node.Name
	fitsNodeAffinity = pluginhelper.PodMatchesNodeSelectorAndAffinityTerms(pod, node)
	_, hasUntoleratedTaint := corev1helpers.FindMatchingUntoleratedTaint(node.Spec.Taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoExecute || t.Effect == corev1.TaintEffectNoSchedule
	})
	fitsTaints = !hasUntoleratedTaint
	return
}

// newDaemonSetPod creates a new pod
func newDaemonSetPod(ds *appsv1.DaemonSet, nodeName string) *corev1.Pod {
	newPod := &corev1.Pod{Spec: ds.Spec.Template.Spec, ObjectMeta: ds.Spec.Template.ObjectMeta}
	newPod.Namespace = ds.Namespace
	newPod.Spec.NodeName = nodeName

	// Added default tolerations for DaemonSet pods.
	util.AddOrUpdateDaemonPodTolerations(&newPod.Spec)

	return newPod
}
