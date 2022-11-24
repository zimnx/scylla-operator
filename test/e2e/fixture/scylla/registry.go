package scylla

import (
	_ "embed"

	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/assets"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string) assets.ObjectTemplate[T] {
	return assets.ParseObjectTemplateOrDie[T](name, tmplString, assets.TemplateFuncs, scheme.Codecs.UniversalDeserializer())
}

var (
	//go:embed "basic.scyllacluster.yaml"
	BasicScyllaCluster ScyllaClusterBytes

	//go:embed "nodeconfig.yaml"
	NodeConfig NodeConfigBytes

	//go:embed "scylladbmonitoring.yaml.tmpl"
	scyllaDBMonitoringTemplateString string
	ScyllaDBMonitoringTemplate       = ParseObjectTemplateOrDie[*scyllav1alpha1.ScyllaDBMonitoring]("scylladbmonitoring", scyllaDBMonitoringTemplateString)

	//go:embed "scylladbmonitoring.ingress.yaml.tmpl"
	scyllaDBMonitoringE2EPrometheusIngressTemplateString string
	ScyllaDBMonitoringE2EPrometheusIngressTemplate       = ParseObjectTemplateOrDie[*networkingv1.Ingress]("scylladbmonitoring-e2e-prometheus-ingress", scyllaDBMonitoringE2EPrometheusIngressTemplateString)
)

type ScyllaClusterBytes []byte

func (sc ScyllaClusterBytes) ReadOrFail() *scyllav1.ScyllaCluster {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1.ScyllaCluster)
}

type NodeConfigBytes []byte

func (sc NodeConfigBytes) ReadOrFail() *scyllav1alpha1.NodeConfig {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1alpha1.NodeConfig)
}
