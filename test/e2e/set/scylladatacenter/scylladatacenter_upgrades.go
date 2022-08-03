// Copyright (C) 2021 ScyllaDB

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter upgrades", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	type entry struct {
		rackSize     int32
		rackCount    int32
		initialImage string
		updatedImage string
	}

	describeEntry := func(e *entry) string {
		return fmt.Sprintf("with %d member(s) and %d rack(s) from %s to %s", e.rackSize, e.rackCount, e.initialImage, e.updatedImage)
	}

	g.DescribeTable("should deploy and update",
		func(e *entry) {
			ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
			defer cancel()

			sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
			sd.Spec.Image = e.initialImage

			o.Expect(sd.Spec.Datacenter.Racks).To(o.HaveLen(1))
			rack := &sd.Spec.Datacenter.Racks[0]
			sd.Spec.Datacenter.Racks = make([]scyllav1alpha1.RackSpec, 0, e.rackCount)
			for i := int32(0); i < e.rackCount; i++ {
				r := rack.DeepCopy()
				r.Name = fmt.Sprintf("rack-%d", i)
				r.Members = pointer.Int32(e.rackSize)
				sd.Spec.Datacenter.Racks = append(sd.Spec.Datacenter.Racks, *r)
			}

			framework.By("Creating a ScyllaDatacenter")
			sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sd.Spec.Image).To(o.Equal(e.initialImage))

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

			framework.By("triggering and update")
			sd, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Patch(
				ctx,
				sd.Name,
				types.MergePatchType,
				[]byte(fmt.Sprintf(
					`{"metadata":{"uid":"%s"},"spec":{"version":"%s"}}`,
					sd.UID,
					e.updatedImage,
				)),
				metav1.PatchOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(sd.Spec.Image).To(o.Equal(e.updatedImage))

			framework.By("Waiting for the ScyllaDatacenter to re-deploy")
			waitCtx2, waitCtx2Cancel := utils.ContextForRollout(ctx, sd)
			defer waitCtx2Cancel()
			sd, err = utils.WaitForScyllaDatacenterState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.IsScyllaDatacenterRolledOut)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = di.UpdateClientEndpoints(ctx, sd)
			o.Expect(err).NotTo(o.HaveOccurred())

			verifyScyllaDatacenter(ctx, f.KubeClient(), sd, di)
		},
		// Test 1 and 3 member rack to cover e.g. handling PDBs correctly.
		g.Entry(describeEntry, &entry{
			rackCount:    1,
			rackSize:     1,
			initialImage: updateFromScyllaImage,
			updatedImage: updateToScyllaImage,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:    1,
			rackSize:     3,
			initialImage: updateFromScyllaImage,
			updatedImage: updateToScyllaImage,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:    1,
			rackSize:     1,
			initialImage: upgradeFromScyllaImage,
			updatedImage: upgradeToScyllaImage,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:    1,
			rackSize:     3,
			initialImage: upgradeFromScyllaImage,
			updatedImage: upgradeToScyllaImage,
		}),
		g.Entry(describeEntry, &entry{
			rackCount:    2,
			rackSize:     3,
			initialImage: upgradeFromScyllaImage,
			updatedImage: upgradeToScyllaImage,
		}),
	)
})
