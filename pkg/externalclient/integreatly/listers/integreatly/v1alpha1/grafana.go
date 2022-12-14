// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// GrafanaLister helps list Grafanas.
// All objects returned here must be treated as read-only.
type GrafanaLister interface {
	// List lists all Grafanas in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Grafana, err error)
	// Grafanas returns an object that can list and get Grafanas.
	Grafanas(namespace string) GrafanaNamespaceLister
	GrafanaListerExpansion
}

// grafanaLister implements the GrafanaLister interface.
type grafanaLister struct {
	indexer cache.Indexer
}

// NewGrafanaLister returns a new GrafanaLister.
func NewGrafanaLister(indexer cache.Indexer) GrafanaLister {
	return &grafanaLister{indexer: indexer}
}

// List lists all Grafanas in the indexer.
func (s *grafanaLister) List(selector labels.Selector) (ret []*v1alpha1.Grafana, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Grafana))
	})
	return ret, err
}

// Grafanas returns an object that can list and get Grafanas.
func (s *grafanaLister) Grafanas(namespace string) GrafanaNamespaceLister {
	return grafanaNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// GrafanaNamespaceLister helps list and get Grafanas.
// All objects returned here must be treated as read-only.
type GrafanaNamespaceLister interface {
	// List lists all Grafanas in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Grafana, err error)
	// Get retrieves the Grafana from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.Grafana, error)
	GrafanaNamespaceListerExpansion
}

// grafanaNamespaceLister implements the GrafanaNamespaceLister
// interface.
type grafanaNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Grafanas in the indexer for a given namespace.
func (s grafanaNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.Grafana, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.Grafana))
	})
	return ret, err
}

// Get retrieves the Grafana from the indexer for a given namespace and name.
func (s grafanaNamespaceLister) Get(name string) (*v1alpha1.Grafana, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("grafana"), name)
	}
	return obj.(*v1alpha1.Grafana), nil
}