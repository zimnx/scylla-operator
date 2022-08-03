// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter replace", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should replace a node", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Datacenter.Racks[0].Members = pointer.Int32(2)

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

		framework.By("Replacing a node #0")
		pod, err := f.KubeClient().CoreV1().Pods(f.Namespace()).Get(
			ctx,
			utils.GetNodeName(sd, 0),
			metav1.GetOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Infof("Initial pod %q has UID is %q", pod.Name, pod.UID)

		_, err = f.KubeClient().CoreV1().Services(f.Namespace()).Patch(
			ctx,
			pod.Name,
			types.MergePatchType,
			[]byte(fmt.Sprintf(
				`{"metadata":{"labels": {"%s": ""}}}`,
				naming.ReplaceLabel,
			)),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the pod to be replaced")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx2Cancel()
		_, err = utils.WaitForPodState(waitCtx2, f.KubeClient().CoreV1().Pods(pod.Namespace), pod.Name, utils.WaitForStateOptions{TolerateDelete: true}, func(p *corev1.Pod) (bool, error) {
			return p.UID != pod.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Give the controller some time to observe that the pod is down.
		time.Sleep(10 * time.Second)

		framework.By("Waiting for the ScyllaDatacenter to re-deploy")
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx3Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx3, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.WaitForStateOptions{}, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)
	})
})
