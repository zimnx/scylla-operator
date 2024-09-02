package scyllaoperatorconfig

import (
	"time"

	"github.com/scylladb/scylla-operator/pkg/gather/collect"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	testTimeout = 15 * time.Minute
)

var (
	socResourceInfo = collect.ResourceInfo{
		Resource: schema.GroupVersionResource{
			Group:    "scylla.scylladb.com",
			Version:  "v1alpha1",
			Resource: "scyllaoperatorconfigs",
		},
		Scope: meta.RESTScopeRoot,
	}
)
