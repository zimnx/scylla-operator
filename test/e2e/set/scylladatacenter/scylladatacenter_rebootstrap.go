// Copyright (c) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should re-bootstrap from old PVCs", func() {
		const membersCount = 3

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Datacenter.Racks[0].Members = pointer.Int32(membersCount)

		framework.By("Creating a ScyllaDatacenter")
		sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalSC := sd.DeepCopy()
		originalSC.ResourceVersion = ""

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

		framework.By("Deleting the ScyllaDatacenter")
		var propagationPolicy = metav1.DeletePropagationForeground
		err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(sd.Namespace).Delete(ctx, sd.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &sd.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		waitCtx2, waitCtx2Cancel := context.WithCancel(ctx)
		defer waitCtx2Cancel()
		err = framework.WaitForObjectDeletion(
			waitCtx2,
			f.DynamicClient(),
			scyllav1.GroupVersion.WithResource("scyllaclusters"),
			sd.Namespace,
			sd.Name,
			&sd.UID,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying PVCs' presence")
		pvcs, err := f.KubeClient().CoreV1().PersistentVolumeClaims(sd.Namespace).List(ctx, metav1.ListOptions{})
		o.Expect(pvcs.Items).To(o.HaveLen(membersCount))
		o.Expect(err).NotTo(o.HaveOccurred())

		pvcMap := map[string]*corev1.PersistentVolumeClaim{}
		for i := range pvcs.Items {
			pvc := &pvcs.Items[i]
			pvcMap[pvc.Name] = pvc
		}

		stsName := naming.StatefulSetNameForRack(sd.Spec.Datacenter.Racks[0], sd)
		for i := int32(0); i < *sd.Spec.Datacenter.Racks[0].Members; i++ {
			podName := fmt.Sprintf("%s-%d", stsName, i)
			pvcName := naming.PVCNameForPod(podName)
			o.Expect(pvcMap).To(o.HaveKey(pvcName))
			o.Expect(pvcMap[pvcName].ObjectMeta.DeletionTimestamp).To(o.BeNil())
		}

		framework.By("Redeploying the ScyllaDatacenter")
		sd = originalSC.DeepCopy()
		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDatacenter to redeploy")
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx3Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx3, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = di.UpdateClientEndpoints(ctx, sd)
		o.Expect(err).NotTo(o.HaveOccurred())
		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)
	})
})
