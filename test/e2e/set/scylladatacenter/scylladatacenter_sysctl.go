// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/tools"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("ScyllaDatacenter sysctl", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should set container sysctl", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		fsAIOMaxNRKey := "fs.aio-max-nr"
		fsAIOMaxNRValue := 2424242 // A unique value.
		o.Expect(sd.Spec.Sysctls).To(o.BeEmpty())
		sd.Spec.Sysctls = []string{fmt.Sprintf("%s=%d", fsAIOMaxNRKey, fsAIOMaxNRValue)}

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

		framework.By("Checking the sysctl value")
		podName := fmt.Sprintf("%s-0", naming.StatefulSetNameForRack(sd.Spec.Datacenter.Racks[0], sd))
		stdout, stderr, err := tools.PodExec(
			f.KubeClient().CoreV1().RESTClient(),
			f.ClientConfig(),
			f.Namespace(),
			podName,
			"scylla",
			[]string{"/usr/sbin/sysctl", "--values", fsAIOMaxNRKey},
			nil,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stderr).To(o.BeEmpty())
		o.Expect(stdout).To(o.Equal(fmt.Sprintf("%d\n", fsAIOMaxNRValue)))
	})
})
