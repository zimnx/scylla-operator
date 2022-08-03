// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter Orphaned PV", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should replace a node with orphaned PV", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Datacenter.Racks[0].Members = pointer.Int32(3)
		sd.Spec.RemoveOrphanedPVs = pointer.Bool(true)

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

		framework.By("Simulating a PV on node that's gone")
		stsName := naming.StatefulSetNameForRack(sd.Spec.Datacenter.Racks[0], sd)
		podName := fmt.Sprintf("%s-%d", stsName, *sd.Spec.Datacenter.Racks[0].Members-1)
		pvcName := naming.PVCNameForPod(podName)

		pvc, err := f.KubeClient().CoreV1().PersistentVolumeClaims(f.Namespace()).Get(ctx, pvcName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pvc.Spec.VolumeName).NotTo(o.BeEmpty())

		pv, err := f.KubeAdminClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		pvCopy := pv.DeepCopy()
		pvCopy.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
			Required: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      corev1.LabelHostname,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"this-node-does-not-exist-42"},
							},
						},
					},
				},
			},
		}

		patchData, err := controllerhelpers.GenerateMergePatch(pv, pvCopy)
		o.Expect(err).NotTo(o.HaveOccurred())

		pv, err = f.KubeAdminClient().CoreV1().PersistentVolumes().Patch(ctx, pv.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pv.Spec.NodeAffinity).NotTo(o.BeNil())

		// We are not listening to PV changes, so we will make a dummy edit on the ScyllaDatacenter.
		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Patch(
			ctx,
			sd.Name,
			types.MergePatchType,
			[]byte(`{"metadata": {"annotations": {"foo": "bar"} } }`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the PVC to be replaced")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx2Cancel()
		pvc, err = utils.WaitForPVCState(waitCtx2, f.KubeClient().CoreV1(), pvc.Namespace, pvc.Name, utils.WaitForStateOptions{TolerateDelete: true}, func(freshPVC *corev1.PersistentVolumeClaim) (bool, error) {
			return freshPVC.UID != pvc.UID, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDatacenter to observe the degradation")
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx3Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx3, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.WaitForStateOptions{}, func(sd *scyllav1alpha1.ScyllaDatacenter) (bool, error) {
			rolledOut, err := utils.IsScyllaDatacenterRolledOut(sd)
			return !rolledOut, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDatacenter to rollout (RV=%s)", sd.ResourceVersion)
		waitCtx4, waitCtx4Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx4Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx4, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.WaitForStateOptions{}, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)
	})
})
