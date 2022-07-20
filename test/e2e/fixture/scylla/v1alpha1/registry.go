// Copyright (c) 2022 ScyllaDB.

package v1

import (
	_ "embed"

	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/scheme"
)

var (
	//go:embed "basic.scylladatacenter.yaml"
	BasicScyllaDatacenter ScyllaDatacenterBytes
)

type ScyllaDatacenterBytes []byte

func (sc ScyllaDatacenterBytes) ReadOrFail() *v1alpha1.ScyllaDatacenter {
	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(sc, nil, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return obj.(*v1alpha1.ScyllaDatacenter)
}
