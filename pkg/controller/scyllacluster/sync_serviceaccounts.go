package scyllacluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (sdc *Controller) syncServiceAccounts(
	ctx context.Context,
	sd *scyllav1alpha1.ScyllaDatacenter,
	serviceAccounts map[string]*corev1.ServiceAccount,
) error {
	requiredServiceAccount, err := MakeServiceAccount(sd, serviceAccounts, sdc.serviceAccountLister)
	if err != nil {
		return fmt.Errorf("can't make service account(s): %w", err)
	}

	// Delete any excessive ServiceAccounts.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, sa := range serviceAccounts {
		if sa.DeletionTimestamp != nil {
			continue
		}

		if sa.Name == requiredServiceAccount.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = sdc.kubeClient.CoreV1().ServiceAccounts(sa.Namespace).Delete(ctx, sa.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &sa.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete service account(s): %w", err)
	}

	_, _, err = resourceapply.ApplyServiceAccount(ctx, sdc.kubeClient.CoreV1(), sdc.serviceAccountLister, sdc.eventRecorder, requiredServiceAccount, true, false)
	if err != nil {
		return fmt.Errorf("can't apply service account: %w", err)
	}

	return nil
}
