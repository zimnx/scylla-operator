// Copyright (c) 2022 ScyllaDB.

package scyllacluster

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (scc *Controller) syncScyllaDatacenters(
	ctx context.Context,
	sc *scyllav2alpha1.ScyllaCluster,
	remoteScyllaDatacenters map[string]map[string]*scyllav1alpha1.ScyllaDatacenter,
	status *scyllav2alpha1.ScyllaClusterStatus,
) error {
	requiredScyllaDatacenters := MakeScyllaDatacenters(sc)

	// Delete any excessive ScyllaDatacenters.
	// Delete has to be the fist action to avoid getting stuck on quota.
	var deletionErrors []error
	for remoteName, scyllaDatacenters := range remoteScyllaDatacenters {
		for _, sd := range scyllaDatacenters {
			if sd.DeletionTimestamp != nil {
				continue
			}

			req, ok := requiredScyllaDatacenters[remoteName]
			if ok && sd.Name == req.Name {
				continue
			}

			propagationPolicy := metav1.DeletePropagationBackground
			var err error
			// TODO: This should first scale all racks to 0, only then remove
			if remoteName != "" {
				dcClient, err := scc.remoteDynamicClient.Region(remoteName)
				if err != nil {
					return fmt.Errorf("can't get client to %q remote cluster: %w", remoteName, err)
				}
				err = dcClient.Resource(scyllaDatacenterGVR).Namespace(sd.Namespace).Delete(ctx, sd.Name, metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						UID: &sd.UID,
					},
					PropagationPolicy: &propagationPolicy,
				})
			} else {
				err = scc.scyllaClient.ScyllaV2alpha1().ScyllaClusters(sd.Namespace).Delete(ctx, sd.Name, metav1.DeleteOptions{
					Preconditions: &metav1.Preconditions{
						UID: &sd.UID,
					},
					PropagationPolicy: &propagationPolicy,
				})
			}

			deletionErrors = append(deletionErrors, err)
		}
	}
	var err error
	err = utilerrors.NewAggregate(deletionErrors)
	if err != nil {
		return fmt.Errorf("can't delete scylla datacenter(s): %w", err)
	}

	for remoteName, sd := range requiredScyllaDatacenters {
		if remoteName == "" {
			sd, _, err = resourceapply.ApplyScyllaDatacenter(ctx, scc.scyllaClient.ScyllaV1alpha1(), scc.scyllaDatacenterLister, scc.eventRecorder, sd)
			if err != nil {
				return fmt.Errorf("can't apply scylla datacenter: %w", err)
			}
		}
		if remoteName != "" {
			remoteClient, err := scc.remoteDynamicClient.Region(remoteName)
			if err != nil {
				return fmt.Errorf("can't get remote client: %w", err)
			}
			sdRemoteClient := remoteClient.Resource(scyllaDatacenterGVR)
			sdRemoteLister := scc.remoteScyllaDatacenterLister.Region(remoteName)

			sd, _, err = resourceapply.ApplyGenericObject(ctx, sdRemoteClient, sdRemoteLister, scc.eventRecorder, sd)
			if err != nil {
				return fmt.Errorf("can't apply scylla datacenter: %w", err)
			}
		}

		for i, dc := range sc.Spec.Datacenters {
			if dc.Name != sd.Spec.DatacenterName {
				continue
			}
			status.Datacenters[i] = *scc.calculateDatacenterStatus(sc, dc, sd)
			break
		}
	}

	return nil
}
