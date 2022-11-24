// Copyright (C) 2022 ScyllaDB

package scylladbmonitoring

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	gapi "github.com/grafana/grafana-api-golang-client"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	prometheusappclient "github.com/prometheus/client_golang/api"
	promeheusappv1api "github.com/prometheus/client_golang/api/prometheus/v1"
	opointer "github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDBMonitoring", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladbmonitoring")

	g.It("should setup monitoring stack", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		sc := scyllafixture.BasicScyllaCluster.ReadOrFail()
		o.Expect(sc.Spec.Datacenter.Racks).To(o.HaveLen(1))
		sc.Spec.Datacenter.Racks[0].Members = 1

		framework.By("Creating a ScyllaCluster with a single node")
		sc, err := f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(
			ctx,
			sc,
			metav1.CreateOptions{
				FieldManager:    f.FieldManager(),
				FieldValidation: metav1.FieldValidationStrict,
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaDBMonitoring")
		sm, _, err := scyllafixture.ScyllaDBMonitoringTemplate.RenderObject(map[string]string{
			"name":              sc.Name,
			"namespace":         sc.Namespace,
			"scyllaClusterName": sc.Name,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		sm, err = f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sc.Namespace).Create(
			ctx,
			sm,
			metav1.CreateOptions{
				FieldManager:    f.FieldManager(),
				FieldValidation: metav1.FieldValidationStrict,
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		prometheusE2EIngress, _, err := scyllafixture.ScyllaDBMonitoringE2EPrometheusIngressTemplate.RenderObject(map[string]string{
			"scyllaDBMonitoringName": sm.Name,
			"namespace":              sm.Namespace,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		prometheusE2EIngress, err = f.KubeClient().NetworkingV1().Ingresses(sm.Namespace).Create(
			ctx,
			prometheusE2EIngress,
			metav1.CreateOptions{
				FieldManager:    f.FieldManager(),
				FieldValidation: metav1.FieldValidationStrict,
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(prometheusE2EIngress.Spec.Rules).To(o.HaveLen(1))
		prometheusServerName := prometheusE2EIngress.Spec.Rules[0].Host

		framework.By("Waiting for the ScyllaCluster to rollout (RV=%s)", sc.ResourceVersion)
		waitCtxL1, waitCtxL1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtxL1Cancel()
		sc, err = utils.WaitForScyllaClusterState(waitCtxL1, f.ScyllaClient().ScyllaV1(), sc.Namespace, sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDBMonitoring to rollout (RV=%s)", sm.ResourceVersion)
		waitCtx2, waitCtx2Cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer waitCtx2Cancel()
		sm, err = utils.WaitForScyllaDBMonitoringState(waitCtx2, f.ScyllaClient().ScyllaV1alpha1().ScyllaDBMonitorings(sc.Namespace), sc.Name, utils.WaitForStateOptions{}, utils.IsScyllaDBMonitoringRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		// We need to retry the prometheus and grafana assertion for several reasons, some of them are:
		//  - ingress exposure is asynchronous and some controllers don't report back status to wait for
		//  - prometheus configuration is asynchronous without any acknowledgement
		//  - grafana configuration is asynchronous without any acknowledgement
		// Some of these may be fixable by manually verifying it in the operator sync loop so it can also be
		// consumed by clients, but it's a bigger effort.

		framework.By("Verifying that Prometheus is configured correctly")

		prometheusServingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-prometheus-serving-ca", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		prometheusServingCACerts, _ := verifyAndParseCABundle(prometheusServingCABundleConfigMap)
		o.Expect(prometheusServingCACerts).To(o.HaveLen(1))

		prometheusServingCAPool := x509.NewCertPool()
		prometheusServingCAPool.AddCert(prometheusServingCACerts[0])

		prometheusGrafanaClientSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-prometheus-client-grafana", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, prometheusGrafanaClientCertBytes, _, prometheusGrafanaClientKeyBytes := verifyAndParseTLSCert(prometheusGrafanaClientSecret, verifyTLSCertOptions{
			isCA:     pointer.Bool(false),
			keyUsage: opointer.KeyUsage(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
		})

		prometheusGrafanaAdminTLSCert, err := tls.X509KeyPair(prometheusGrafanaClientCertBytes, prometheusGrafanaClientKeyBytes)
		o.Expect(err).NotTo(o.HaveOccurred())

		promHTTPClient, err := prometheusappclient.NewClient(prometheusappclient.Config{
			Address: "https://" + f.GetIngressAddress(prometheusServerName),
			Client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						ServerName:   prometheusServerName,
						Certificates: []tls.Certificate{prometheusGrafanaAdminTLSCert},
						RootCAs:      prometheusServingCAPool,
					},
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					ForceAttemptHTTP2:     true,
					MaxIdleConns:          100,
					IdleConnTimeout:       90 * time.Second,
					TLSHandshakeTimeout:   10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		promClient := promeheusappv1api.NewAPI(promHTTPClient)

		o.Eventually(func(eo o.Gomega) {
			ctxTargets, ctxTargetsCancel := context.WithTimeout(ctx, 15*time.Second)
			defer ctxTargetsCancel()

			targets, err := promClient.Targets(ctxTargets)
			framework.Infof("Listing grafana targets: err: %v, active: %d, dropped: %d", err, len(targets.Active), len(targets.Dropped))
			eo.Expect(err).NotTo(o.HaveOccurred())

			eo.Expect(targets.Active).To(o.HaveLen(1))
			for _, t := range targets.Active {
				eo.Expect(t.Health).To(o.Equal(promeheusappv1api.HealthGood))
			}

			// TODO: Uncomment. There shouldn't be any dropped targets. Currently, /service-discovery contains
			//       "undefined (0 / 57 active targets)" that are in addition to our ServiceMonitor definition.
			//       (Maciek was looking into this, it seems to be a bug in prometheus operator.)
			// o.Expect(targets.Dropped).To(o.HaveLen(0))
		}).WithTimeout(5 * 60 * time.Second).WithPolling(1 * time.Second).Should(o.Succeed())

		framework.By("Verifying that Grafana is configured correctly")

		grafanaAccessCredentialsSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana-access-credentials", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(grafanaAccessCredentialsSecret.Data).To(o.HaveLen(2))
		o.Expect(grafanaAccessCredentialsSecret.Data).To(o.HaveKey("username"))
		o.Expect(grafanaAccessCredentialsSecret.Data).To(o.HaveKey("password"))

		grafanaUsername := string(grafanaAccessCredentialsSecret.Data["username"])
		o.Expect(grafanaUsername).NotTo(o.BeEmpty())
		grafanaPassword := string(grafanaAccessCredentialsSecret.Data["password"])
		o.Expect(grafanaPassword).NotTo(o.BeEmpty())

		grafanaServingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-grafana-serving-ca", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		grafanaServingCACerts, _ := verifyAndParseCABundle(grafanaServingCABundleConfigMap)
		o.Expect(grafanaServingCACerts).To(o.HaveLen(1))

		grafanaServingCAPool := x509.NewCertPool()
		grafanaServingCAPool.AddCert(grafanaServingCACerts[0])

		o.Expect(sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress.DNSDomains).To(o.HaveLen(1))
		grafanaServerName := sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress.DNSDomains[0]

		grafanaClient, err := gapi.New(
			"https://"+f.GetIngressAddress(grafanaServerName),
			gapi.Config{
				BasicAuth: url.UserPassword(grafanaUsername, grafanaPassword),
				Client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							ServerName: grafanaServerName,
							RootCAs:    grafanaServingCAPool,
						},
						Proxy: http.ProxyFromEnvironment,
						DialContext: (&net.Dialer{
							Timeout:   30 * time.Second,
							KeepAlive: 30 * time.Second,
						}).DialContext,
						ForceAttemptHTTP2:     true,
						MaxIdleConns:          100,
						IdleConnTimeout:       90 * time.Second,
						TLSHandshakeTimeout:   10 * time.Second,
						ExpectContinueTimeout: 1 * time.Second,
					},
					Timeout: 15 * time.Second,
				},
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedDashboards := []gapi.FolderDashboardSearchResponse{
			{
				ID:          2,
				Title:       "CQL Overview",
				URI:         "db/cql-overview",
				Slug:        "",
				Type:        "dash-db",
				Tags:        []string{},
				IsStarred:   false,
				FolderID:    1,
				FolderTitle: "scylladb",
			},
		}

		var dashboards []gapi.FolderDashboardSearchResponse
		o.Eventually(func(eo o.Gomega) {
			dashboards, err = grafanaClient.Dashboards()
			framework.Infof("Listing grafana dashboards: err: %v, count: %d", err, len(dashboards))
			eo.Expect(err).NotTo(o.HaveOccurred())
			eo.Expect(dashboards).To(o.HaveLen(len(expectedDashboards)))
		}).WithTimeout(10 * 60 * time.Second).WithPolling(1 * time.Second).Should(o.Succeed())

		// Clear random fields for comparison.
		for i := range dashboards {
			d := &dashboards[i]
			d.UID = ""
			d.URL = ""
			d.FolderUID = ""
			d.FolderURL = ""
		}
		o.Expect(dashboards).To(o.Equal(expectedDashboards))
	})
})
