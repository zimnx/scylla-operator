package nodeconfigdaemon

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (ncdc *Controller) pruneJobs(ctx context.Context, jobs map[string]*batchv1.Job, requiredJobs []*batchv1.Job) error {
	var errs []error
	for _, j := range jobs {
		if j.DeletionTimestamp != nil {
			continue
		}

		isRequired := false
		for _, req := range requiredJobs {
			if j.Name == req.Name && j.Namespace == req.Namespace {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		klog.InfoS("Removing stale Job", "Job", klog.KObj(j))
		propagationPolicy := metav1.DeletePropagationBackground
		err := ncdc.kubeClient.BatchV1().Jobs(j.Namespace).Delete(ctx, j.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &j.UID,
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
