package scylla

import (
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "prometheus.grafanadatasource.yaml.tmpl"
	prometheusGrafanaDataSourceTemplateString string
	PrometheusGrafanaDataSourceTemplate       = ParseObjectTemplateOrDie[*integreatlyv1alpha1.GrafanaDataSource]("prometheus.grafanadatasource", prometheusGrafanaDataSourceTemplateString)
)
