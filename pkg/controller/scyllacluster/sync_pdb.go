package scyllacluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (sdc *Controller) syncPodDisruptionBudgets(
	ctx context.Context,
	sd *scyllav1alpha1.ScyllaDatacenter,
	status *scyllav1alpha1.ScyllaDatacenterStatus,
	pdbs map[string]*policyv1.PodDisruptionBudget,
) (*scyllav1alpha1.ScyllaDatacenterStatus, error) {
	var err error

	requiredPDB := MakePodDisruptionBudget(sd)

	// Delete any excessive PodDisruptionBudgets.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for _, pdb := range pdbs {
		if pdb.DeletionTimestamp != nil {
			continue
		}

		if pdb.Name == requiredPDB.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err = sdc.kubeClient.PolicyV1().PodDisruptionBudgets(pdb.Namespace).Delete(ctx, pdb.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &pdb.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		deletionErrors = append(deletionErrors, err)
	}
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return status, fmt.Errorf("can't delete pdb(s): %w", err)
	}

	// TODO: Remove forced ownership in v1.5 (#672)
	_, _, err = resourceapply.ApplyPodDisruptionBudget(ctx, sdc.kubeClient.PolicyV1(), sdc.pdbLister, sdc.eventRecorder, requiredPDB, true)
	if err != nil {
		return status, fmt.Errorf("can't apply pdb: %w", err)
	}

	return status, nil
}
