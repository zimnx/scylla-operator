package v1alpha1

import (
	_ "embed"

	"github.com/scylladb/scylla-operator/pkg/assets"
	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "grafana.yaml"
	grafanaTemplateString string
	GrafanaTemplate       = ParseObjectTemplateOrDie[*integreatlyv1alpha1.Grafana]("grafana", grafanaTemplateString)
)
