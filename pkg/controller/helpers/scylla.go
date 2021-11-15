package helpers

import (
	"fmt"
	"sort"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scyllaclient"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/errors"
	corev1schedulinghelpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
)

func GetScyllaIPFromService(svc *corev1.Service) (string, error) {
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		return "", fmt.Errorf("service %s is of type %q instead of %q", naming.ObjRef(svc), svc.Spec.Type, corev1.ServiceTypeClusterIP)
	}

	if svc.Spec.ClusterIP == corev1.ClusterIPNone {
		return "", fmt.Errorf("service %s doesn't have a ClusterIP", naming.ObjRef(svc))
	}

	return svc.Spec.ClusterIP, nil
}

func GetScyllaHost(statefulsetName string, ordinal int32, services map[string]*corev1.Service) (string, error) {
	svcName := fmt.Sprintf("%s-%d", statefulsetName, ordinal)
	svc, found := services[svcName]
	if !found {
		return "", fmt.Errorf("missing service %q", svcName)
	}

	ip, err := GetScyllaIPFromService(svc)
	if err != nil {
		return "", err
	}

	return ip, nil
}

func GetRequiredScyllaHosts(sc *scyllav1.ScyllaCluster, services map[string]*corev1.Service) ([]string, error) {
	var hosts []string
	var errs []error
	for _, rack := range sc.Spec.Datacenter.Racks {
		for ord := int32(0); ord < rack.Members; ord++ {
			stsName := naming.StatefulSetNameForRack(rack, sc)
			host, err := GetScyllaHost(stsName, ord, services)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			hosts = append(hosts, host)
		}
	}
	var err error = errors.NewAggregate(errs)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

func NewScyllaClientFromToken(hosts []string, authToken string) (*scyllaclient.Client, error) {
	// TODO: unify logging
	logger, _ := log.NewProduction(log.Config{
		Level: zap.NewAtomicLevelAt(zapcore.InfoLevel),
	})

	cfg := scyllaclient.DefaultConfig(authToken, hosts...)
	scyllaClient, err := scyllaclient.NewClient(cfg, logger.Named("scylla_client"))
	if err != nil {
		return nil, err
	}

	return scyllaClient, nil
}

func NewScyllaClientFromSecret(secret *corev1.Secret, hosts []string) (*scyllaclient.Client, error) {
	token, err := helpers.GetAgentAuthTokenFromSecret(secret)
	if err != nil {
		return nil, err
	}

	return NewScyllaClientFromToken(hosts, token)
}

func SetRackCondition(rackStatus *scyllav1.RackStatus, newCondition scyllav1.RackConditionType) {
	for i := range rackStatus.Conditions {
		if rackStatus.Conditions[i].Type == newCondition {
			rackStatus.Conditions[i].Status = corev1.ConditionTrue
			return
		}
	}
	rackStatus.Conditions = append(
		rackStatus.Conditions,
		scyllav1.RackCondition{Type: newCondition, Status: corev1.ConditionTrue},
	)
}

func FindNodeStatus(nodeStatuses []v1alpha1.NodeStatus, nodeName string) *v1alpha1.NodeStatus {
	for i := range nodeStatuses {
		ns := &nodeStatuses[i]
		if ns.Name == nodeName {
			return ns
		}
	}

	return nil
}

func SetNodeStatus(nodeStatuses []v1alpha1.NodeStatus, status *v1alpha1.NodeStatus) []v1alpha1.NodeStatus {
	for i, ns := range nodeStatuses {
		if ns.Name == status.Name {
			nodeStatuses[i] = *status
			return nodeStatuses
		}
	}

	nodeStatuses = append(nodeStatuses, *status)

	sort.SliceStable(nodeStatuses, func(i, j int) bool {
		return nodeStatuses[i].Name < nodeStatuses[j].Name
	})

	return nodeStatuses
}

func IsNodeConfigSelectingNode(nc *v1alpha1.NodeConfig, node *corev1.Node) (bool, error) {
	// TODO: split into dedicated functions.
	// Check nodeSelector.
	if !labels.SelectorFromSet(nc.Spec.Placement.NodeSelector).Matches(labels.Set(node.Labels)) {
		return false, nil
	}

	// Check affinity.

	affinityNodeSelector, err := nodeaffinity.NewNodeSelector(
		nc.Spec.Placement.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
	)
	if err != nil {
		return false, fmt.Errorf("can't construct node affinity node selector: %w", err)
	}

	if !affinityNodeSelector.Match(node) {
		return false, nil
	}

	// Check taints and tolerations.

	_, isUntolerated := corev1schedulinghelpers.FindMatchingUntoleratedTaint(
		node.Spec.Taints,
		nc.Spec.Placement.Tolerations,
		func(t *corev1.Taint) bool {
			// We are only interested in NoSchedule and NoExecute taints.
			return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
		},
	)
	if !isUntolerated {
		return false, nil
	}

	return true, nil
}

func IsNodeTunedForContainer(nc *v1alpha1.NodeConfig, nodeName string, containerID string) bool {
	ns := FindNodeStatus(nc.Status.NodeStatuses, nodeName)
	if ns == nil {
		return false
	}

	if !ns.TunedNode {
		return false
	}

	return true
}
