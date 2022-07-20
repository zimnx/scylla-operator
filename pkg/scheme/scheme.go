package scheme

import (
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	Scheme = runtime.NewScheme()
)

func init() {
	err := scheme.AddToScheme(Scheme)
	if err != nil {
		panic(err)
	}

	err = scyllav1.Install(Scheme)
	if err != nil {
		panic(err)
	}

	err = scyllav1alpha1.Install(Scheme)
	if err != nil {
		panic(err)
	}

	err = scyllav2alpha1.Install(Scheme)
	if err != nil {
		panic(err)
	}
}
