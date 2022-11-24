package v1alpha1

import (
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "grafana.yaml"
	grafanaTemplateString string
	GrafanaTemplate       = ParseObjectTemplateOrDie[*integreatlyv1alpha1.Grafana]("grafana", grafanaTemplateString)

	//go:embed "grafana-access-credentials.secret.yaml"
	grafanaAccessCredentialsSecretTemplateString string
	GrafanaAccessCredentialsSecretTemplate       = ParseObjectTemplateOrDie[*corev1.Secret]("grafana-access-credentials-secret", grafanaAccessCredentialsSecretTemplateString)

	//go:embed "scylladb.grafanafolder.yaml"
	grafanaScyllaDBFolderTemplateString string
	GrafanaScyllaDBFolderTemplate       = ParseObjectTemplateOrDie[*integreatlyv1alpha1.GrafanaFolder]("grafanafolder-scylladb", grafanaScyllaDBFolderTemplateString)

	//go:embed "overview-dashboard.configmap.yaml"
	grafanaOverviewDashboardConfigMapTemplateString string
	GrafanaOverviewDashboardConfigMapTemplate       = ParseObjectTemplateOrDie[*corev1.ConfigMap]("grafana-overview-dashboard-cm", grafanaOverviewDashboardConfigMapTemplateString)

	//go:embed "overview.grafanadashboard.yaml"
	grafanaOverviewDashboardTemplateString string
	GrafanaOverviewDashboardTemplate       = ParseObjectTemplateOrDie[*integreatlyv1alpha1.GrafanaDashboard]("grafana-overview-dashboard", grafanaOverviewDashboardTemplateString)

	//go:embed "prometheus.grafanadatasource.yaml"
	grafanaPrometheusDatasourceTemplateString string
	GrafanaPrometheusDatasourceTemplate       = ParseObjectTemplateOrDie[*integreatlyv1alpha1.GrafanaDataSource]("grafana-prometheus-datasource", grafanaPrometheusDatasourceTemplateString)

	//go:embed "grafana.ingress.yaml"
	grafanaIngressTemplateString string
	GrafanaIngressTemplate       = ParseObjectTemplateOrDie[*networkingv1.Ingress]("grafana-ingress", grafanaIngressTemplateString)
)
