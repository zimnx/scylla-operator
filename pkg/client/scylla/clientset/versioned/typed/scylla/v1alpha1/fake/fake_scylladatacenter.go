// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeScyllaDatacenters implements ScyllaDatacenterInterface
type FakeScyllaDatacenters struct {
	Fake *FakeScyllaV1alpha1
	ns   string
}

var scylladatacentersResource = schema.GroupVersionResource{Group: "scylla.scylladb.com", Version: "v1alpha1", Resource: "scylladatacenters"}

var scylladatacentersKind = schema.GroupVersionKind{Group: "scylla.scylladb.com", Version: "v1alpha1", Kind: "ScyllaDatacenter"}

// Get takes name of the scyllaDatacenter, and returns the corresponding scyllaDatacenter object, and an error if there is any.
func (c *FakeScyllaDatacenters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ScyllaDatacenter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(scylladatacentersResource, c.ns, name), &v1alpha1.ScyllaDatacenter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ScyllaDatacenter), err
}

// List takes label and field selectors, and returns the list of ScyllaDatacenters that match those selectors.
func (c *FakeScyllaDatacenters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ScyllaDatacenterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(scylladatacentersResource, scylladatacentersKind, c.ns, opts), &v1alpha1.ScyllaDatacenterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ScyllaDatacenterList{ListMeta: obj.(*v1alpha1.ScyllaDatacenterList).ListMeta}
	for _, item := range obj.(*v1alpha1.ScyllaDatacenterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested scyllaDatacenters.
func (c *FakeScyllaDatacenters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(scylladatacentersResource, c.ns, opts))

}

// Create takes the representation of a scyllaDatacenter and creates it.  Returns the server's representation of the scyllaDatacenter, and an error, if there is any.
func (c *FakeScyllaDatacenters) Create(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.CreateOptions) (result *v1alpha1.ScyllaDatacenter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(scylladatacentersResource, c.ns, scyllaDatacenter), &v1alpha1.ScyllaDatacenter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ScyllaDatacenter), err
}

// Update takes the representation of a scyllaDatacenter and updates it. Returns the server's representation of the scyllaDatacenter, and an error, if there is any.
func (c *FakeScyllaDatacenters) Update(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.UpdateOptions) (result *v1alpha1.ScyllaDatacenter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(scylladatacentersResource, c.ns, scyllaDatacenter), &v1alpha1.ScyllaDatacenter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ScyllaDatacenter), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeScyllaDatacenters) UpdateStatus(ctx context.Context, scyllaDatacenter *v1alpha1.ScyllaDatacenter, opts v1.UpdateOptions) (*v1alpha1.ScyllaDatacenter, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(scylladatacentersResource, "status", c.ns, scyllaDatacenter), &v1alpha1.ScyllaDatacenter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ScyllaDatacenter), err
}

// Delete takes name of the scyllaDatacenter and deletes it. Returns an error if one occurs.
func (c *FakeScyllaDatacenters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(scylladatacentersResource, c.ns, name), &v1alpha1.ScyllaDatacenter{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeScyllaDatacenters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(scylladatacentersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ScyllaDatacenterList{})
	return err
}

// Patch applies the patch and returns the patched scyllaDatacenter.
func (c *FakeScyllaDatacenters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ScyllaDatacenter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(scylladatacentersResource, c.ns, name, pt, data, subresources...), &v1alpha1.ScyllaDatacenter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ScyllaDatacenter), err
}
