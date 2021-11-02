// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/klog/v2"
)

func (ncdc *Controller) makeJobsForNode(ctx context.Context) ([]*batchv1.Job, error) {
	pod, err := ncdc.selfPodLister.Pods(ncdc.namespace).Get(ncdc.podName)
	if err != nil {
		return nil, fmt.Errorf("can't get Pod %s/%s: %w", ncdc.namespace, ncdc.podName, err)
	}

	var jobs []*batchv1.Job

	jobs = append(jobs, makePerftuneJobForNode(
		ncdc.newControllerRef(),
		ncdc.namespace,
		ncdc.nodeName,
		ncdc.scyllaImage,
		&pod.Spec,
	))

	return jobs, nil
}

func (ncdc *Controller) syncJobsForNode(ctx context.Context, jobs map[string]*batchv1.Job, nodeStatus *v1alpha1.NodeStatus) error {
	required, err := ncdc.makeJobsForNode(ctx)
	if err != nil {
		return fmt.Errorf("can't make Jobs: %w", err)
	}

	err = ncdc.pruneJobs(ctx, jobs, required)
	if err != nil {
		return fmt.Errorf("can't prune Jobs: %w", err)
	}

	finished := true
	for _, j := range required {
		updatedJob, _, err := resourceapply.ApplyJob(ctx, ncdc.kubeClient.BatchV1(), ncdc.namespacedJobLister, ncdc.eventRecorder, j)
		if err != nil {
			return fmt.Errorf("can't create job %s: %w", naming.ObjRef(j), err)
		}

		// FIXME: Extract into a function and double check how jobs report status.
		if updatedJob.Status.CompletionTime != nil && updatedJob.Status.Succeeded > 0 {
			klog.V(4).InfoS("Job isn't completed yet", "Job", klog.KObj(updatedJob))
			finished = false
		}
	}

	nodeStatus.TunedNode = finished

	return nil
}
