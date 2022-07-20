// Copyright (c) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
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

		framework.By("Rejecting a creation of ScyllaDatacenter with duplicated racks")
		duplicateRacksSD := validSD.DeepCopy()
		duplicateRacksSD.Spec.Racks = append(duplicateRacksSD.Spec.Racks, *duplicateRacksSD.Spec.Racks[0].DeepCopy())
		_, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, duplicateRacksSD, metav1.CreateOptions{})
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDatacenter.scylla.scylladb.com %q is invalid: spec.racks[1].name: Duplicate value: "us-east-1a"`, duplicateRacksSD.Name),
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
						Field:   "spec.racks[1].name",
					},
				},
			},
			Code: 422,
		}}))

		framework.By("Accepting a creation of valid ScyllaCluster")
		validSD, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, validSD, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", validSD.ResourceVersion)
		waitCtx1, waitCtx1Cancel := v1alpha1.ContextForRollout(ctx, validSD)
		defer waitCtx1Cancel()
		validSD, err = v1alpha1.WaitForScyllaDatacenterState(waitCtx1, f.ScyllaClient().ScyllaV1alpha1(), validSD.Namespace, validSD.Name, v1alpha1.WaitForStateOptions{}, v1alpha1.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), validSD, v1alpha1.GetMemberCount(validSD))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), validSD, di)

		framework.By("Rejecting an update of ScyllaDatacenter's datacenter name")
		_, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(validSD.Namespace).Patch(
			ctx,
			validSD.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(`{"spec": {"datacenterName": "%s-updated"}}`, validSD.Spec.DatacenterName)),
			metav1.PatchOptions{},
		)
		o.Expect(err).To(o.Equal(&errors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf(`admission webhook "webhook.scylla.scylladb.com" denied the request: ScyllaDatacenter.scylla.scylladb.com %q is invalid: spec.datacenterName: Forbidden: change of datacenter name is currently not supported`, validSD.Name),
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
						Field:   "spec.datacenterName",
					},
				},
			},
			Code: 422,
		}}))
	})
})
