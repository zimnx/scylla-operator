// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should support scaling", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Datacenter.Racks[0].Members = pointer.Int32(1)

		framework.By("Creating a ScyllaDatacenter with 1 member")
		sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDatacenter to rollout")
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx1Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx1, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		di, err := NewDataInserter(ctx, f.KubeClient().CoreV1(), sd, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)

		framework.By("Scaling the ScyllaDatacenter to 3 replicas")
		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Patch(
			ctx,
			sd.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sd.Spec.Datacenter.Racks).To(o.HaveLen(1))
		o.Expect(sd.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(3))

		framework.By("Waiting for the ScyllaDatacenter to rollout")
		waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx2Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)

		di, err = NewDataInserter(ctx, f.KubeClient().CoreV1(), sd, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer di.Close()

		err = di.Insert()
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Scaling the ScyllaDatacenter down to 2 replicas")
		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(sd.Namespace).Patch(
			ctx,
			sd.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 2}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sd.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(2))

		framework.By("Waiting for the ScyllaDatacenter to rollout")
		waitCtx3, waitCtx3Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx3Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx3, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)

		podName := naming.StatefulSetNameForRack(sd.Spec.Datacenter.Racks[0], sd) + "-1"
		svcName := podName
		framework.By("Marking ScyllaDatacenter node #2 (%s) for maintenance", podName)
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					naming.NodeMaintenanceLabel: "",
				},
			},
		}
		patch, err := helpers.CreateTwoWayMergePatch(&corev1.Service{}, svc)
		o.Expect(err).NotTo(o.HaveOccurred())
		svc, err = f.KubeClient().CoreV1().Services(sd.Namespace).Patch(
			ctx,
			svcName,
			types.StrategicMergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Manually draining ScyllaDatacenter node #2 (%s)", podName)
		ec := &corev1.EphemeralContainer{
			TargetContainerName: naming.ScyllaContainerName,
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:            "e2e-drain-scylla",
				Image:           sd.Spec.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/usr/bin/nodetool", "drain"},
				Args:            []string{},
			},
		}
		pod, err := utils.RunEphemeralContainerAndWaitForCompletion(ctx, f.KubeClient().CoreV1().Pods(sd.Namespace), podName, ec)
		o.Expect(err).NotTo(o.HaveOccurred())
		ephemeralContainerState := controllerhelpers.FindContainerStatus(pod, ec.Name)
		o.Expect(ephemeralContainerState).NotTo(o.BeNil())
		o.Expect(ephemeralContainerState.State.Terminated).NotTo(o.BeNil())
		o.Expect(ephemeralContainerState.State.Terminated.ExitCode).To(o.BeEquivalentTo(0))

		framework.By("Scaling the ScyllaDatacenter down to 1 replicas")
		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Patch(
			ctx,
			sd.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 1}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sd.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(1))

		framework.By("Waiting for the ScyllaDatacenter to rollout")
		waitCtx5, waitCtx5Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx5Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx5, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)

		framework.By("Scaling the ScyllaDatacenter back to 3 replicas to make sure there isn't an old (decommissioned) storage in place")
		sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Patch(
			ctx,
			sd.Name,
			types.JSONPatchType,
			[]byte(`[{"op": "replace", "path": "/spec/datacenter/racks/0/members", "value": 3}]`),
			metav1.PatchOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(sd.Spec.Datacenter.Racks[0].Members).To(o.BeEquivalentTo(3))

		framework.By("Waiting for the ScyllaDatacenter to rollout")
		waitCtx6, waitCtx6Cancel := utils.ContextForRollout(ctx, sd)
		defer waitCtx6Cancel()
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx6, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)
	})
})
