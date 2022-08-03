// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"encoding/json"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("Scylla Manager integration", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should discover cluster and sync tasks", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Datacenter.Racks[0].Members = 1
		sd.Spec.Repairs = append(sd.Spec.Repairs, scyllav1alpha1.RepairTaskSpec{
			SchedulerTaskSpec: scyllav1alpha1.SchedulerTaskSpec{
				Name:     "weekly repair",
				Interval: "7d",
			},
			Parallel: 123,
		})

		// Backup scheduling is going to fail because location is not accessible in our env.
		sd.Spec.Backups = append(sd.Spec.Backups, scyllav1alpha1.BackupTaskSpec{
			SchedulerTaskSpec: scyllav1alpha1.SchedulerTaskSpec{
				Name:     "daily backup",
				Interval: "1d",
			},
			Location: []string{"s3:bucket"},
		})

		framework.By("Creating a ScyllaDatacenter")
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

		framework.By("Waiting for the cluster sync with Scylla Manager")
		registeredInManagerCond := func(sd *scyllav1alpha1.ScyllaDatacenter) (bool, error) {
			return sd.Status.ManagerID != nil, nil
		}

		repairTaskScheduledCond := func(sd *scyllav1alpha1.ScyllaDatacenter) (bool, error) {
			for _, r := range sd.Status.Repairs {
				if r.Name == sd.Spec.Repairs[0].Name {
					return r.ID != "", nil
				}
			}
			return false, nil
		}

		backupTaskSyncFailedCond := func(sd *scyllav1alpha1.ScyllaDatacenter) (bool, error) {
			for _, b := range sd.Status.Backups {
				if b.Name == sd.Spec.Backups[0].Name {
					return b.Error != "", nil
				}
			}
			return false, nil
		}

		waitCtx2, waitCtx2Cancel := utils.ContextForManagerSync(ctx, sd)
		defer waitCtx2Cancel()
		conditions := []func(sd *scyllav1alpha1.ScyllaDatacenter) (bool, error){
			registeredInManagerCond,
			repairTaskScheduledCond,
			backupTaskSyncFailedCond,
		}
		sd, err = utils.WaitForScyllaDatacenterState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, conditions...)
		o.Expect(err).NotTo(o.HaveOccurred())

		managerClient, err := utils.GetManagerClient(ctx, f.KubeAdminClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying that task properties were synchronized")
		tasks, err := managerClient.ListTasks(ctx, *sd.Status.ManagerID, "repair", false, "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tasks.ExtendedTaskSlice).To(o.HaveLen(1))
		repairTask := tasks.ExtendedTaskSlice[0]
		o.Expect(repairTask.Name).To(o.Equal(sd.Spec.Repairs[0].Name))
		o.Expect(repairTask.Properties.(map[string]interface{})["parallel"].(json.Number).Int64()).To(o.Equal(int64(123)))
	})
})
