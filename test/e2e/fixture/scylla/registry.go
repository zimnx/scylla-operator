package scylla

import (
	_ "embed"

	o "github.com/onsi/gomega"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
)

var (
	//go:embed "basic.scylladatacenter.yaml"
	BasicScyllaDatacenter ScyllaDatacenterBytes

	//go:embed "nodeconfig.yaml"
	NodeConfig NodeConfigBytes
)

type ScyllaDatacenterBytes []byte

func (sc ScyllaDatacenterBytes) ReadOrFail() *scyllav1alpha1.ScyllaDatacenter {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1alpha1.ScyllaDatacenter)
}

type NodeConfigBytes []byte

func (sc NodeConfigBytes) ReadOrFail() *scyllav1alpha1.NodeConfig {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*scyllav1alpha1.NodeConfig)
}
