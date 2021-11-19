// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

const (
	nodeConfigRolloutTimeout = 5 * time.Minute
)

// These tests modify global resource affecting global cluster state.
// They must not be run asynchronously with other tests.
var _ = g.Describe("NodeConfig Optimizations [Serial]", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("nodeconfig")

	ncTemplate := scyllafixture.NodeConfig.ReadOrFail()

	preconditionSuccessful := false
	g.JustBeforeEach(func() {
		// Make sure the NodeConfig is not present.
		framework.By("Making sure NodeConfig %q, doesn't exist", naming.ObjRef(ncTemplate))
		_, err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Get(context.Background(), ncTemplate.Name, metav1.GetOptions{})
		if err == nil {
			framework.Failf("NodeConfig %q can't be present before running this test", naming.ObjRef(ncTemplate))
		} else if !apierrors.IsNotFound(err) {
			framework.Failf("Can't get NodeConfig %q: %v", naming.ObjRef(ncTemplate), err)
		}

		preconditionSuccessful = true
	})

	g.JustAfterEach(func() {
		if !preconditionSuccessful {
			return
		}

		framework.By("Deleting NodeConfig %q, if it exists", naming.ObjRef(ncTemplate))
		err := f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Delete(context.Background(), ncTemplate.Name, metav1.DeleteOptions{})
		if err != nil {
			framework.Failf("Can't delete NodeConfig %q: %v", naming.ObjRef(ncTemplate), err)
		}

		err = framework.WaitForObjectDeletion(context.Background(), f.DynamicAdminClient(), scyllav1alpha1.GroupVersion.WithResource("nodeconfigs"), ncTemplate.Namespace, ncTemplate.Name, nil)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should create tuning resources and tune nodes", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		nc := ncTemplate.DeepCopy()

		g.By("Verifying there is at least one scylla node")
		nodeList, err := f.KubeAdminClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var matchingNodes []*corev1.Node
		for i := range nodeList.Items {
			node := &nodeList.Items[i]
			isSelectingNode, err := controllerhelpers.IsNodeConfigSelectingNode(nc, node)
			o.Expect(err).NotTo(o.HaveOccurred())

			if isSelectingNode {
				matchingNodes = append(matchingNodes, node)
			}
		}
		o.Expect(matchingNodes).NotTo(o.HaveLen(0))
		framework.Infof("There are %d scylla nodes", len(matchingNodes))

		g.By("Creating a NodeConfig")
		nc, err = f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs().Create(ctx, nc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func(name string, uid types.UID) {

		}(nc.Name, nc.UID)

		framework.By("Waiting for the NodeConfig to deploy")
		waitCtx1, waitCtx1Cancel := context.WithTimeout(ctx, nodeConfigRolloutTimeout)
		defer waitCtx1Cancel()
		nc, err = utils.WaitForNodeConfigState(waitCtx1, f.ScyllaAdminClient().ScyllaV1alpha1().NodeConfigs(), nc.Name, utils.WaitForStateOptions{TolerateDelete: false}, utils.IsNodeConfigRolledOut, utils.IsNodeConfigDoneWithNodeTuningFunc(matchingNodes))
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyNodeConfig(ctx, f.KubeAdminClient(), nc)

		// There should be a tuning job for every scylla node.
		nodeJobList, err := f.KubeAdminClient().BatchV1().Jobs(naming.ScyllaOperatorNodeTuningNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				naming.NodeConfigNameLabel:    nc.Name,
				naming.NodeConfigJobTypeLabel: string(naming.NodeConfigJobTypeNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeJobList.Items).To(o.HaveLen(len(matchingNodes)))

		var jobNodeNames []string
		for _, j := range nodeJobList.Items {
			o.Expect(j.Annotations).NotTo(o.BeNil())
			nodeName, found := j.Annotations[naming.NodeConfigJobForNodeKey]
			o.Expect(found).To(o.BeTrue())

			o.Expect(nodeName).NotTo(o.BeEmpty())
			jobNodeNames = append(jobNodeNames, nodeName)
		}

		var matchingNodeNames []string
		for _, node := range matchingNodes {
			matchingNodeNames = append(matchingNodeNames, node.Name)
		}
	})
})
