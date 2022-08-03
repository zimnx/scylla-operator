// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"encoding/json"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func addQuantity(lhs resource.Quantity, rhs resource.Quantity) *resource.Quantity {
	res := lhs.DeepCopy()
	res.Add(rhs)

	// Pre-cache the string so DeepEqual works.
	_ = res.String()

	return &res
}

var _ = g.Describe("ScyllaDatacenter", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should reconcile resource changes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		framework.By("Creating a ScyllaDatacenter")
		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		o.Expect(sd.Spec.Datacenter.Racks).To(o.HaveLen(1))

		sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDatacenter to deploy")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx1Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx1, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sd, utils.GetMemberCount(sd))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)

		framework.By("Changing pod resources")
		oldResources := *sd.Spec.Datacenter.Racks[0].ScyllaContainer.Resources.DeepCopy()
		newResources := *oldResources.DeepCopy()
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Requests).To(o.HaveKey(corev1.ResourceMemory))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceCPU))
		o.Expect(oldResources.Limits).To(o.HaveKey(corev1.ResourceMemory))

		newResources.Requests[corev1.ResourceCPU] = *addQuantity(newResources.Requests[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Requests[corev1.ResourceMemory] = *addQuantity(newResources.Requests[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Requests).NotTo(o.BeEquivalentTo(oldResources.Requests))

		newResources.Limits[corev1.ResourceCPU] = *addQuantity(newResources.Limits[corev1.ResourceCPU], resource.MustParse("1m"))
		newResources.Limits[corev1.ResourceMemory] = *addQuantity(newResources.Limits[corev1.ResourceMemory], resource.MustParse("1Mi"))
		o.Expect(newResources.Limits).NotTo(o.BeEquivalentTo(oldResources.Limits))

		o.Expect(newResources).NotTo(o.BeEquivalentTo(oldResources))

		newResourcesJSON, err := json.Marshal(newResources)
		o.Expect(err).NotTo(o.HaveOccurred())

		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Patch(
			ctx,
			sd.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/scyllaContainer/resources", "value": %s}]`,
				newResourcesJSON,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sd.Spec.Datacenter.Racks[0].ScyllaContainer.Resources).To(o.BeEquivalentTo(newResources))

		framework.By("Waiting for the ScyllaDatacenter to redeploy")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx2Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Scaling the ScyllaDatacenter up to create a new replica")
		oldMembers := *sd.Spec.Datacenter.Racks[0].Members
		newMebmers := oldMembers + 1
		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Patch(
			ctx,
			sd.Name,
			types.JSONPatchType,
			[]byte(fmt.Sprintf(
				`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": %d}]`,
				newMebmers,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sd.Spec.Datacenter.Racks[0].Members).To(o.Equal(newMebmers))

		framework.By("Waiting for the ScyllaDatacenter to redeploy")
		waitCtx4, waitCtx4Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx4Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx4, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
