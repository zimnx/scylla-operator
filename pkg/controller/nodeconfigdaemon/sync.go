// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncdc *Controller) getCanAdoptFunc(ctx context.Context) func() error {
	return func() error {
		fresh, err := ncdc.scyllaClient.ScyllaV1alpha1().NodeConfigs().Get(ctx, ncdc.nodeConfigName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != ncdc.nodeConfigUID {
			return fmt.Errorf("original NodeConfig %v is gone: got uid %v, wanted %v", ncdc.nodeConfigName, fresh.UID, ncdc.nodeConfigUID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v has just been deleted at %v", fresh.Name, fresh.DeletionTimestamp)
		}

		return nil
	}
}

func (ncdc *Controller) getJobs(ctx context.Context, selector labels.Selector) (map[string]*batchv1.Job, error) {
	// List all Job to find even those that no longer match our selector.
	// They will be orphaned in ClaimJob().
	jobs, err := ncdc.namespacedJobLister.Jobs(ncdc.namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("can't list Jobs: %w", err)
	}

	cm := controllertools.NewJobControllerRefManager(
		ctx,
		&metav1.ObjectMeta{
			Name:              ncdc.nodeConfigName,
			UID:               ncdc.nodeConfigUID,
			DeletionTimestamp: nil,
		},
		controllerGVK,
		selector,
		ncdc.getCanAdoptFunc(ctx),
		controllertools.RealJobControl{
			KubeClient: ncdc.kubeClient,
			Recorder:   ncdc.eventRecorder,
		},
	)

	claimedJobs, err := cm.ClaimJobs(jobs)
	if err != nil {
		return nil, fmt.Errorf("can't claim jobs in %q namespace, %w", ncdc.namespace, err)
	}

	return claimedJobs, nil
}

func (ncdc *Controller) getJobsForNode(ctx context.Context) (map[string]*batchv1.Job, error) {
	return ncdc.getJobs(
		ctx,
		labels.SelectorFromSet(labels.Set{
			naming.NodeConfigJobForNodeLabel: ncdc.nodeName,
		}),
	)
}

func (ncdc *Controller) sync(ctx context.Context, key string) error {
	// // Cache contains only Pods running on the same Node so it's cheap.
	// scyllaPods, err := c.podLister.Pods(corev1.NamespaceAll).List(naming.ScyllaSelector())
	// if err != nil {
	// 	return fmt.Errorf("can't list all Pods on the node: %w", err)
	// }
	//
	// // Sanity check.
	// var errs []error
	// for _, pod := range scyllaPods {
	// 	if pod.Spec.NodeName != c.nodeName {
	// 		errs = append(errs, fmt.Errorf("pod"))
	// 	}
	// }

	// klog.V(4).Infof("There are %d Scylla Pods running on the node", len(scyllaPods))

	nodeJobs, err := ncdc.getJobsForNode(ctx)
	if err != nil {
		fmt.Errorf("can't get Jobs: %w", err)
	}

	// We configure the node first. Doing it serially avoid the need to check the node
	// configuration status as it always precedes the pod level configuration.
	err = ncdc.syncJobsForNode(ctx, nodeJobs)

	// jobs, err := c.getJobs(ctx, snc)
	// if err != nil {
	// 	return fmt.Errorf("can't get Jobs: %w", err)
	// }
	//
	var errs []error
	// if err = c.syncJobs(ctx, snc, scyllaPods, jobs); err != nil {
	// 	errs = append(errs, fmt.Errorf("can't sync Jobs: %w", err))
	// }

	return utilerrors.NewAggregate(errs)
}
