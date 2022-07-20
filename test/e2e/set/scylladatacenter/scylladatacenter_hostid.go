// Copyright (c) 2022 ScyllaDB

package scylladatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter HostID", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should be reflected as a Service annotation", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Racks[0].Nodes = pointer.Int32(2)

		framework.By("Creating a ScyllaDatacenter")
		sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaDatacenter to deploy")
		waitCtx, waitCtxCancel := v1alpha1.ContextForRollout(ctx, sd)
		defer waitCtxCancel()

		sd, err = v1alpha1.WaitForScyllaDatacenterState(waitCtx, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, v1alpha1.WaitForStateOptions{}, v1alpha1.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sd, v1alpha1.GetMemberCount(sd))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)

		framework.By("Verifying annotations")
		scyllaClient, _, err := v1alpha1.GetScyllaClient(ctx, f.KubeClient().CoreV1(), sd)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer scyllaClient.Close()

		svcs, err := f.KubeClient().CoreV1().Services(sd.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: v1alpha1.GetMemberServiceSelector(sd.Name).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, svc := range svcs.Items {
			host := svc.Spec.ClusterIP
			o.Expect(host).NotTo(o.Equal(corev1.ClusterIPNone))

			hostID, err := scyllaClient.GetLocalHostId(ctx, host, false)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(hostID).NotTo(o.BeEmpty())

			o.Expect(svc.Annotations).To(o.HaveKeyWithValue(naming.HostIDAnnotation, hostID))
		}
	})
})
