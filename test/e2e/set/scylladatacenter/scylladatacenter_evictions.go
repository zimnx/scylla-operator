// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaDatacenter evictions", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should allow one disruption", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Datacenter.Racks[0].Members = 2

		framework.By("Creating a ScyllaDatacenter")
		sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		
		framework.By("Waiting for the ScyllaDatacenter to rollout (RV=%s)", sd.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx1Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx1, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.WaitForStateOptions{}, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sd, utils.GetMemberCount(sd))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)

		framework.By("Allowing the first pod to be evicted")
		e := &policyv1beta1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.GetNodeName(sd, 0),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).Evict(ctx, e)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Forbidding to evict a second pod")
		e = &policyv1beta1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.GetNodeName(sd, 1),
				Namespace: f.Namespace(),
			},
		}
		err = f.KubeAdminClient().CoreV1().Pods(f.Namespace()).Evict(ctx, e)
		o.Expect(err).Should(o.MatchError("Cannot evict pod as it would violate the pod's disruption budget."))
	})
})
