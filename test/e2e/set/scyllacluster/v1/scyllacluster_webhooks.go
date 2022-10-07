// Copyright (c) 2021-2022 ScyllaDB.

package v1

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
)

var _ = g.Describe("ScyllaCluster webhook", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.It("should forbid invalid requests", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		validSC := scyllafixture.BasicScyllaCluster.ReadOrFail()
		validSC.Name = names.SimpleNameGenerator.GenerateName(validSC.GenerateName)

		framework.By("rejecting a creation of ScyllaCluster with duplicated racks")
		duplicateRacksSC := validSC.DeepCopy()
		duplicateRacksSC.Spec.Datacenter.Racks = append(duplicateRacksSC.Spec.Datacenter.Racks, *duplicateRacksSC.Spec.Datacenter.Racks[0].DeepCopy())
		_, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, duplicateRacksSC, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaCluster.scylla.scylladb.com %q is invalid: spec.datacenter.racks[1].name: Duplicate value: "us-east-1a"`, duplicateRacksSC.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  duplicateRacksSC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaCluster",
				UID:   "",
				Causes: []metav1.StatusCause{
					{
						Type:    "FieldValueDuplicate",
						Message: `Duplicate value: "us-east-1a"`,
						Field:   "spec.datacenter.racks[1].name",
					},
				},
			},
			Code: 422,
		}}))

		framework.By("rejecting a creation of ScyllaCluster with invalid intensity in repair task spec")
		invalidIntensitySC := validSC.DeepCopy()
		invalidIntensitySC.Spec.Repairs = append(invalidIntensitySC.Spec.Repairs, scyllav1.RepairTaskSpec{
			Intensity: "100Mib",
		})
		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, invalidIntensitySC, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaCluster.scylla.scylladb.com %q is invalid: spec.repairs[0].intensity: Invalid value: "100Mib": invalid intensity, it must be a float value`, invalidIntensitySC.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  invalidIntensitySC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaCluster",
				UID:   "",
				Causes: []metav1.StatusCause{
					{
						Type:    "FieldValueInvalid",
						Message: `Invalid value: "100Mib": invalid intensity, it must be a float value`,
						Field:   "spec.repairs[0].intensity",
					},
				},
			},
			Code: 422,
		}}))

		framework.By("accepting a creation of valid ScyllaCluster")
		validSC, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, validSC, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("waiting for the ScyllaCluster to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, validSC)
		defer waitCtx1Cancel()
		validSC, err = utils.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1(), validSC.Namespace, validSC.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), validSC, utils.GetMemberCount(validSC))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), f.ScyllaClient(), validSC, di)

		framework.By("rejecting an update of ScyllaCluster's repo")
		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(validSC.Namespace).Patch(
			ctx,
			validSC.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec": {"datacenter": {"name": "%s-updated"}}}`, validSC.Spec.Datacenter.Name)),
			metav1.PatchOptions{},
		)
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaCluster.scylla.scylladb.com %q is invalid: spec.datacenter.name: Forbidden: change of datacenter name is currently not supported`, validSC.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  validSC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaCluster",
				UID:   "",
				Causes: []metav1.StatusCause{
					{
						Type:    "FieldValueForbidden",
						Message: "Forbidden: change of datacenter name is currently not supported",
						Field:   "spec.datacenter.name",
					},
				},
			},
			Code: 422,
		}}))

		framework.By("rejecting an update of ScyllaCluster's dcName")
		_, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(validSC.Namespace).Patch(
			ctx,
			validSC.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec":{"datacenter": {"name": "%s-updated"}}}`, validSC.Spec.Datacenter.Name)),
			metav1.PatchOptions{},
		)
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaCluster.scylla.scylladb.com %q is invalid: spec.datacenter.name: Forbidden: change of datacenter name is currently not supported`, validSC.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  validSC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaCluster",
				UID:   "",
				Causes: []metav1.StatusCause{
					{
						Type:    "FieldValueForbidden",
						Message: "Forbidden: change of datacenter name is currently not supported",
						Field:   "spec.datacenter.name",
					},
				},
			},
			Code: 422,
		}}))
	})
})
