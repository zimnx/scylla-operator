// Copyright (c) 2022 ScyllaDB.

package v1

import (
	"context"
	"fmt"
	"sort"
	"strings"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils/v1"
	appsv1 "k8s.io/api/apps/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/pointer"
)

func verifyPersistentVolumeClaims(ctx context.Context, coreClient corev1client.CoreV1Interface, sc *scyllav1.ScyllaCluster) {
	pvcList, err := coreClient.PersistentVolumeClaims(sc.Namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Infof("Found %d pvc(s) in namespace %q", len(pvcList.Items), sc.Namespace)

	pvcNamePrefix := fmt.Sprintf("%s-%s-", naming.PVCTemplateName, sc.Name)

	var scPVCNames []string
	for _, pvc := range pvcList.Items {
		if pvc.DeletionTimestamp != nil {
			framework.Infof("pvc %s is being deleted", naming.ObjRef(&pvc))
			continue
		}

		if !strings.HasPrefix(pvc.Name, pvcNamePrefix) {
			framework.Infof("pvc %s doesn't match the prefix %q", naming.ObjRef(&pvc), pvcNamePrefix)
			continue
		}

		scPVCNames = append(scPVCNames, pvc.Name)
	}
	framework.Infof("Found %d pvc(s) for ScyllaCluster %q", len(scPVCNames), naming.ObjRef(sc))

	var expectedPvcNames []string
	for _, rack := range sc.Spec.Datacenter.Racks {
		for ord := int32(0); ord < rack.Members; ord++ {
			stsName := fmt.Sprintf("%s-%s-%s", sc.Name, sc.Spec.Datacenter.Name, rack.Name)
			expectedPvcNames = append(expectedPvcNames, naming.PVCNameForStatefulSet(stsName, ord))
		}
	}

	sort.Strings(scPVCNames)
	sort.Strings(expectedPvcNames)
	o.Expect(scPVCNames).To(o.BeEquivalentTo(expectedPvcNames))
}

func verifyStatefulset(sts *appsv1.StatefulSet) {
	o.Expect(sts.DeletionTimestamp).To(o.BeNil())
	o.Expect(sts.Status.ObservedGeneration).To(o.Equal(sts.Generation))
	o.Expect(sts.Spec.Replicas).NotTo(o.BeNil())
	o.Expect(sts.Status.ReadyReplicas).To(o.Equal(*sts.Spec.Replicas))
	o.Expect(sts.Status.CurrentRevision).To(o.Equal(sts.Status.UpdateRevision))
}

func verifyPodDisruptionBudget(sd *scyllav1alpha1.ScyllaDatacenter, pdb *policyv1beta1.PodDisruptionBudget) {
	o.Expect(pdb.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v1alpha1",
				Kind:               "ScyllaDatacenter",
				Name:               sd.Name,
				UID:                sd.UID,
				BlockOwnerDeletion: pointer.Bool(true),
				Controller:         pointer.Bool(true),
			},
		}),
	)
	clusterSelector := naming.ScyllaLabels()
	clusterSelector[naming.ClusterNameLabel] = sd.Name
	o.Expect(pdb.Spec.MaxUnavailable.IntValue()).To(o.Equal(1))
	o.Expect(pdb.Spec.Selector).To(o.Equal(metav1.SetAsLabelSelector(clusterSelector)))
}

func verifyScyllaCluster(ctx context.Context, kubeClient kubernetes.Interface, scyllaClient scyllaclient.Interface, sc *scyllav1.ScyllaCluster, di *DataInserter) {
	framework.By("Verifying the ScyllaCluster")

	o.Expect(sc.Status.ObservedGeneration).NotTo(o.BeNil())
	o.Expect(sc.Status.Racks).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	sd, err := scyllaClient.ScyllaV1alpha1().ScyllaDatacenters(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sd.ObjectMeta.OwnerReferences).To(o.BeEquivalentTo(
		[]metav1.OwnerReference{
			{
				APIVersion:         "scylla.scylladb.com/v2alpha1",
				Kind:               "ScyllaCluster",
				Name:               sc.Name,
				UID:                sc.UID,
				BlockOwnerDeletion: pointer.Bool(true),
				Controller:         pointer.Bool(true),
			},
		}),
	)

	statefulsets, err := v1.GetStatefulSetsForScyllaCluster(ctx, kubeClient.AppsV1(), sd)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(statefulsets).To(o.HaveLen(len(sc.Spec.Datacenter.Racks)))

	memberCount := 0
	for _, r := range sc.Spec.Datacenter.Racks {
		memberCount += int(r.Members)

		s := statefulsets[r.Name]

		verifyStatefulset(s)

		o.Expect(sc.Status.Racks[r.Name].Stale).NotTo(o.BeNil())
		o.Expect(*sc.Status.Racks[r.Name].Stale).To(o.BeFalse())
		o.Expect(sc.Status.Racks[r.Name].ReadyMembers).To(o.Equal(r.Members))
		o.Expect(sc.Status.Racks[r.Name].ReadyMembers).To(o.Equal(s.Status.ReadyReplicas))
		o.Expect(sc.Status.Racks[r.Name].UpdatedMembers).NotTo(o.BeNil())
		o.Expect(*sc.Status.Racks[r.Name].UpdatedMembers).To(o.Equal(s.Status.UpdatedReplicas))
	}

	if sc.Status.Upgrade != nil {
		o.Expect(sc.Status.Upgrade.FromVersion).To(o.Equal(sc.Status.Upgrade.ToVersion))
	}

	pdb, err := kubeClient.PolicyV1beta1().PodDisruptionBudgets(sc.Namespace).Get(ctx, sc.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	verifyPodDisruptionBudget(sd, pdb)

	verifyPersistentVolumeClaims(ctx, kubeClient.CoreV1(), sc)

	// TODO: Use scylla client to check at least "UN"
	client, hosts, err := v1.GetScyllaClient(ctx, kubeClient.CoreV1(), sc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hosts).To(o.HaveLen(memberCount))

	status, err := client.Status(ctx, "")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(status.DownHosts()).To(o.BeEmpty())
	o.Expect(status.LiveHosts()).To(o.HaveLen(memberCount))

	if di != nil {
		read, err := di.Read()
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Verifying data consistency")
		o.Expect(read).To(o.Equal(di.GetExpected()))
	}
}
