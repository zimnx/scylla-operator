// Copyright (c) 2022 ScyllaDB

package scyllacluster

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/gocql/gocql/scyllacloud"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/gocqlx/v2"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/features"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"github.com/scylladb/scylla-operator/pkg/scylla/api/cqlclient/v1alpha1"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

var _ = g.Describe("ScyllaCluster Ingress", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scyllacluster")

	g.FIt("should be able to connect to cluster via Ingresses", func() {
		var skipMessages []string
		if !utilfeature.DefaultMutableFeatureGate.Enabled(features.AutomaticTLSCertificates) {
			skipMessages = append(skipMessages, fmt.Sprintf("%q feature is disabled", features.AutomaticTLSCertificates))
		}
		if len(framework.TestContext.IngressController.IngressClassName) == 0 {
			skipMessages = append(skipMessages, "ingress controller ingress class name isn't provided")
		}
		if len(framework.TestContext.IngressController.Address) == 0 {
			skipMessages = append(skipMessages, "ingress controller address isn't provided")
		}
		if len(skipMessages) != 0 {
			g.Skip(fmt.Sprintf("Skipping because %s", strings.Join(skipMessages, " and ")))
		}

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.DNSDomains = []string{"private.nodes.scylladb.com"}
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			CQL: &scyllav1.CQLExposeOptions{
				Ingress: &scyllav1.IngressOptions{
					IngressClassName: framework.TestContext.IngressController.IngressClassName,
					Annotations:      framework.TestContext.IngressController.CustomAnnotations,
				},
			},
		}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		hosts := getScyllaHostsAndWaitForFullQuorum(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(hosts).To(o.HaveLen(1))

		framework.By("Injecting ingress controller address into the CQL connection bundle")
		bundleSecret, err := f.KubeClient().CoreV1().Secrets(sc.Namespace).Get(ctx, naming.GetScyllaClusterLocalAdminConnectionConfigName(sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		decoder := scheme.Codecs.DecoderToVersion(scheme.DefaultYamlSerializer, schema.GroupVersions(scheme.Scheme.PrioritizedVersionsAllGroups()))
		bundleRaw, err := runtime.Decode(decoder, bundleSecret.Data["config"])
		bundle := bundleRaw.(*v1alpha1.ConnectionConfig)

		bundle.Datacenters[sc.Spec.Datacenter.Name].Server = framework.TestContext.IngressController.Address

		spew.Dump(bundle)

		encoder := scheme.Codecs.EncoderForVersion(scheme.DefaultYamlSerializer, schema.GroupVersions(scheme.Scheme.PrioritizedVersionsAllGroups()))
		bundleData, err := runtime.Encode(encoder, bundle)
		o.Expect(err).NotTo(o.HaveOccurred())

		bundleFile, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("%s-", f.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(bundleFile.Name())

		err = bundleFile.Close()
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Saving CQL Connection bundle in %s", bundleFile.Name())
		err = os.WriteFile(bundleFile.Name(), bundleData, 0600)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Connecting to cluster via Ingress Controller")
		cluster, err := scyllacloud.NewCloudCluster(bundleFile.Name())
		o.Expect(err).NotTo(o.HaveOccurred())

		session, err := gocqlx.WrapSession(cluster.CreateSession())
		o.Expect(err).NotTo(o.HaveOccurred())

		di := insertAndVerifyCQLData(ctx, hosts, utils.WithSession(&session))
		defer di.Close()

		verifyCQLData(ctx, di)
	})

	g.It("should create ingress objects when ingress exposeOptions are provided", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.DNSDomains = []string{"private.nodes.scylladb.com", "public.nodes.scylladb.com"}
		sc.Spec.ExposeOptions = &scyllav1.ExposeOptions{
			CQL: &scyllav1.CQLExposeOptions{
				Ingress: &scyllav1.IngressOptions{
					IngressClassName: "my-cql-ingress-class",
					Annotations: map[string]string{
						"my-cql-annotation-key": "my-cql-annotation-value",
					},
				},
			},
		}

		framework.By("Creating a ScyllaCluster")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxCancel()

		sc, err = utils.WaitForScyllaClusterState(waitCtx, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaCluster(ctx, f.KubeClient(), sc)

		framework.By("Verifying AnyNode Ingresses")
		services, err := f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeIdentity),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services.Items).To(o.HaveLen(1))

		ingresses, err := f.KubeClient().NetworkingV1().Ingresses(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeAnyNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				"any.cql.private.nodes.scylladb.com",
				"any.cql.public.nodes.scylladb.com",
			},
			"cql-ssl",
		)

		framework.By("Verifying Node Ingresses")
		services, err = f.KubeClient().CoreV1().Services(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services.Items).To(o.HaveLen(1))

		var nodeHostIDs []string
		for _, svc := range services.Items {
			o.Expect(svc.Annotations).To(o.HaveKey(naming.HostIDAnnotation))
			nodeHostIDs = append(nodeHostIDs, svc.Annotations[naming.HostIDAnnotation])
		}

		ingresses, err = f.KubeClient().NetworkingV1().Ingresses(sc.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sc.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sc.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				fmt.Sprintf("%s.cql.private.nodes.scylladb.com", nodeHostIDs[0]),
				fmt.Sprintf("%s.cql.public.nodes.scylladb.com", nodeHostIDs[0]),
			},
			"cql-ssl",
		)
	})
})

func verifyIngress(ingress *networkingv1.Ingress, name string, annotations map[string]string, className string, service *corev1.Service, hosts []string, servicePort string) {
	o.Expect(ingress.Name).To(o.Equal(name))
	for k, v := range annotations {
		o.Expect(ingress.Annotations).To(o.HaveKeyWithValue(k, v))
	}

	o.Expect(ingress.Spec.IngressClassName).ToNot(o.BeNil())
	o.Expect(*ingress.Spec.IngressClassName).To(o.Equal(className))
	o.Expect(ingress.Spec.Rules).To(o.HaveLen(len(hosts)))
	for i := range hosts {
		o.Expect(ingress.Spec.Rules[i].Host).To(o.Equal(hosts[i]))
	}
	for _, rule := range ingress.Spec.Rules {
		o.Expect(rule.HTTP.Paths).To(o.HaveLen(1))
		o.Expect(rule.HTTP.Paths[0].Path).To(o.Equal("/"))
		o.Expect(rule.HTTP.Paths[0].PathType).ToNot(o.BeNil())
		o.Expect(*rule.HTTP.Paths[0].PathType).To(o.Equal(networkingv1.PathTypePrefix))
		o.Expect(rule.HTTP.Paths[0].Backend.Service).ToNot(o.BeNil())
		o.Expect(rule.HTTP.Paths[0].Backend.Service.Name).To(o.Equal(service.Name))
		o.Expect(rule.HTTP.Paths[0].Backend.Service.Port.Name).To(o.Equal(servicePort))
	}
}
