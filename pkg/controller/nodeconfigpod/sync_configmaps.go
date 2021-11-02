// Copyright (C) 2021 ScyllaDB

package nodeconfigpod

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncpc *Controller) pruneConfigMaps(ctx context.Context, required *corev1.ConfigMap, configMaps map[string]*corev1.ConfigMap) error {
	var errs []error
	for _, ds := range configMaps {
		if ds.DeletionTimestamp != nil {
			continue
		}

		if ds.Name == required.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncpc.kubeClient.CoreV1().ConfigMaps(ds.Namespace).Delete(ctx, ds.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &ds.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (ncpc *Controller) syncConfigMaps(
	ctx context.Context,
	pod *corev1.Pod,
	configMaps map[string]*corev1.ConfigMap,
) error {

	// FIXME: list NodeConfigs, Nodes, match and sync status
	data := map[string]string{}

	required := makeConfigMap(pod, data)

	// Delete any excessive ConfigMaps.
	// Delete has to be the first action to avoid getting stuck on quota.
	err := ncpc.pruneConfigMaps(ctx, required, configMaps)
	if err != nil {
		return fmt.Errorf("can't delete DaemonSet(s): %w", err)
	}

	if required != nil {
		_, _, err := resourceapply.ApplyConfigMap(ctx, ncpc.kubeClient.CoreV1(), ncpc.configMapLister, ncpc.eventRecorder, required)
		if err != nil {
			return fmt.Errorf("can't apply ConfigMap: %w", err)
		}
	}

	return nil
}
