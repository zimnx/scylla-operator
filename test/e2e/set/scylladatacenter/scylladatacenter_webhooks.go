// Copyright (c) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"
)

var _ = g.Describe("ScyllaDatacenter webhook", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should forbid invalid requests", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		validSD := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		validSD.Name = names.SimpleNameGenerator.GenerateName(validSD.GenerateName)

		framework.By("rejecting a creation of ScyllaDatacenter with duplicated racks")
		duplicateRacksSD := validSD.DeepCopy()
		duplicateRacksSD.Spec.Datacenter.Racks = append(duplicateRacksSD.Spec.Datacenter.Racks, *duplicateRacksSD.Spec.Datacenter.Racks[0].DeepCopy())
		_, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, duplicateRacksSD, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDatacenter.scylla.scylladb.com %q is invalid: spec.datacenter.racks[1].name: Duplicate value: "us-east-1a"`, duplicateRacksSD.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  duplicateRacksSD.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaDatacenter",
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

		framework.By("rejecting a creation of ScyllaDatacenter with invalid intensity in repair task spec")
		invalidIntensitySC := validSD.DeepCopy()
		invalidIntensitySC.Spec.Repairs = append(invalidIntensitySC.Spec.Repairs, scyllav1alpha1.RepairTaskSpec{
			Intensity: "100Mib",
		})
		_, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, invalidIntensitySC, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDatacenter.scylla.scylladb.com %q is invalid: spec.repairs[0].intensity: Invalid value: "100Mib": invalid intensity, it must be a float value`, invalidIntensitySC.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  invalidIntensitySC.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaDatacenter",
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

		framework.By("accepting a creation of valid ScyllaDatacenter")
		validSD, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, validSD, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("waiting for the ScyllaDatacenter to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, validSD)
		defer waitCtx1Cancel()
		validSD, err = utils.WaitForScyllaDatacenterState(waitCtx1, f.ScyllaClient().ScyllaV1alpha1(), validSD.Namespace, validSD.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), validSD, utils.GetMemberCount(validSD))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), validSD, di)

		framework.By("rejecting an update of ScyllaDatacenter's repo")
		_, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(validSD.Namespace).Patch(
			ctx,
			validSD.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec": {"datacenter": {"name": "%s-updated"}}}`, validSD.Spec.Datacenter.Name)),
			metav1.PatchOptions{},
		)
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDatacenter.scylla.scylladb.com %q is invalid: spec.datacenter.name: Forbidden: change of datacenter name is currently not supported`, validSD.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  validSD.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaDatacenter",
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

		framework.By("rejecting an update of ScyllaDatacenter's dcName")
		_, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(validSD.Namespace).Patch(
			ctx,
			validSD.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec":{"datacenter": {"name": "%s-updated"}}}`, validSD.Spec.Datacenter.Name)),
			metav1.PatchOptions{},
		)
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDatacenter.scylla.scylladb.com %q is invalid: spec.datacenter.name: Forbidden: change of datacenter name is currently not supported`, validSD.Name),
			Reason:  "Invalid",
			Details: &metav1.StatusDetails{
				Name:  validSD.Name,
				Group: "scylla.scylladb.com",
				Kind:  "ScyllaDatacenter",
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
