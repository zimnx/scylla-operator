package controllerhelpers

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type PruneControlInterface interface {
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

type PruneControlFuncs struct {
	DeleteFunc func(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func (pcf *PruneControlFuncs) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return pcf.DeleteFunc(ctx, name, opts)
}

var _ PruneControlInterface = &PruneControlFuncs{}

func Prune[T metav1.Object](ctx context.Context, requiredObjects []T, existingObjects map[string]T, control PruneControlInterface) error {
	var errs []error

	for _, existing := range existingObjects {
		if existing.GetDeletionTimestamp() != nil {
			continue
		}

		isRequired := false
		for _, required := range requiredObjects {
			if existing.GetName() == required.GetName() {
				isRequired = true
				break
			}
		}
		if isRequired {
			continue
		}

		uid := existing.GetUID()
		propagationPolicy := metav1.DeletePropagationBackground
		err := control.Delete(ctx, existing.GetName(), metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &uid,
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
