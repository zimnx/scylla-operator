// Copyright (c) 2022 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	utils "github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter Ingress", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should create ingress objects when ingress exposeOptions are provided", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllafixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Racks[0].Nodes = pointer.Int32(1)
		sd.Spec.DNSDomains = []string{"private.nodes.scylladb.com", "public.nodes.scylladb.com"}
		sd.Spec.ExposeOptions = &scyllav1alpha1.ExposeOptions{
			CQL: &scyllav1alpha1.CQLExposeOptions{
				Ingress: &scyllav1alpha1.IngressOptions{
					IngressClassName: "my-cql-ingress-class",
					Annotations: map[string]string{
						"my-cql-annotation-key": "my-cql-annotation-value",
					},
				},
			},
		}

		framework.By("Creating a ScyllaDatacenter")
		sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO: In theory Ingresses may not be created when ScyllaCluster is rolled out making this test flaky.
		//       Wait for condition when it's available.
		framework.By("Waiting for ScyllaCluster to deploy")
		waitCtx, waitCtxCancel := utils.ContextForRollout(ctx, sd)
		defer waitCtxCancel()

		sd, err = utils.WaitForScyllaDatacenterState(waitCtx, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, utils.WaitForStateOptions{}, utils.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		verifyScyllaDatacenter(ctx, f.KubeClient(), sd, nil)

		framework.By("Verifying AnyNode Ingresses")
		services, err := f.KubeClient().CoreV1().Services(sd.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeIdentity),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(services.Items).To(o.HaveLen(1))

		ingresses, err := f.KubeClient().NetworkingV1().Ingresses(sd.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeAnyNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sd.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sd.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				"any.cql.private.nodes.scylladb.com",
				"any.cql.public.nodes.scylladb.com",
			},
			"cql-ssl",
		)

		framework.By("Verifying Node Ingresses")
		services, err = f.KubeClient().CoreV1().Services(sd.Namespace).List(ctx, metav1.ListOptions{
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

		ingresses, err = f.KubeClient().NetworkingV1().Ingresses(sd.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				naming.ScyllaIngressTypeLabel: string(naming.ScyllaIngressTypeNode),
			}).String(),
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ingresses.Items).To(o.HaveLen(1))

		verifyIngress(
			&ingresses.Items[0],
			fmt.Sprintf("%s-cql", services.Items[0].Name),
			sd.Spec.ExposeOptions.CQL.Ingress.Annotations,
			sd.Spec.ExposeOptions.CQL.Ingress.IngressClassName,
			&services.Items[0],
			[]string{
				fmt.Sprintf("%s.cql.private.nodes.scylladb.com", nodeHostIDs[0]),
				fmt.Sprintf("%s.cql.public.nodes.scylladb.com", nodeHostIDs[0]),
			},
			"cql-ssl",
		)

		// TODO: extend the test with a connection to ScyllaCluster via these Ingresses.
		//       Ref: https://github.com/scylladb/scylla-operator/issues/1015
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
