// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
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
	startTime := time.Now()
	klog.V(4).InfoS("Started sync", "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished sync", "duration", time.Since(startTime))
	}()

	nodeJobs, err := ncdc.getJobsForNode(ctx)
	if err != nil {
		return fmt.Errorf("can't get Jobs: %w", err)
	}

	// We configure the node first. Doing it serially avoids the need to check the node
	// configuration status as it always precedes the pod level configuration.
	jobsForNodeFinished, err := ncdc.syncJobsForNode(ctx, nodeJobs)
	if err != nil {
		return fmt.Errorf("can't sync jobs for node: %w", err)
	}

	if !jobsForNodeFinished {
		klog.V(4).InfoS("Waiting for node jobs to finish")
		return nil
	}

	// jobs, err := c.getJobs(ctx, snc)
	// if err != nil {
	// 	return fmt.Errorf("can't get Jobs: %w", err)
	// }

	var errs []error
	// if err = c.syncJobs(ctx, snc, scyllaPods, jobs); err != nil {
	// 	errs = append(errs, fmt.Errorf("can't sync Jobs: %w", err))
	// }

	return utilerrors.NewAggregate(errs)
}
