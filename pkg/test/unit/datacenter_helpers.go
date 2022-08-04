package unit

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// NewSingleRackDatacenter returns ScyllaDatacenter having single racks.
func NewSingleRackDatacenter(members int32) *scyllav1alpha1.ScyllaDatacenter {
	return NewDetailedSingleRackDatacenter("test-datacenter", "test-ns", "repo:2.3.1", "test-dc", "test-rack", members)
}

// NewMultiRackDatacenter returns ScyllaDatacenter having multiple racks.
func NewMultiRackDatacenter(members ...int32) *scyllav1alpha1.ScyllaDatacenter {
	return NewDetailedMultiRackDatacenter("test-datacenter", "test-ns", "repo:2.3.1", "test-dc", members...)
}

// NewDetailedSingleRackDatacenter returns ScyllaDatacenter having single rack with supplied information.
func NewDetailedSingleRackDatacenter(name, namespace, image, dc, rack string, members int32) *scyllav1alpha1.ScyllaDatacenter {
	return &scyllav1alpha1.ScyllaDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: scyllav1alpha1.ScyllaDatacenterSpec{
			Image: image,
			Datacenter: scyllav1alpha1.DatacenterSpec{
				Name: dc,
				Racks: []scyllav1alpha1.RackSpec{
					{
						Name:    rack,
						Members: pointer.Int32(members),
						Storage: scyllav1alpha1.StorageSpec{
							Capacity: "5Gi",
						},
						ScyllaContainer: scyllav1alpha1.ScyllaContainerSpec{
							ContainerSpec: scyllav1alpha1.ContainerSpec{
								Resources: corev1.ResourceRequirements{
									Limits: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceCPU:    resource.MustParse("2"),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
								},
							},
						},
					},
				},
			},
		},
		Status: scyllav1alpha1.ScyllaDatacenterStatus{
			ObservedGeneration: pointer.Int64(1),
			Racks: map[string]scyllav1alpha1.RackStatus{
				rack: {
					Image:        image,
					Members:      pointer.Int32(members),
					ReadyMembers: pointer.Int32(members),
					Stale:        pointer.Bool(false),
				},
			},
		},
	}
}

// NewDetailedMultiRackDatacenter creates multi rack database with supplied information.
func NewDetailedMultiRackDatacenter(name, namespace, image, dc string, members ...int32) *scyllav1alpha1.ScyllaDatacenter {
	c := &scyllav1alpha1.ScyllaDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: 1,
		},
		Spec: scyllav1alpha1.ScyllaDatacenterSpec{
			Image: image,
			Datacenter: scyllav1alpha1.DatacenterSpec{
				Name:  dc,
				Racks: []scyllav1alpha1.RackSpec{},
			},
		},
		Status: scyllav1alpha1.ScyllaDatacenterStatus{
			ObservedGeneration: pointer.Int64(1),
			Racks:              map[string]scyllav1alpha1.RackStatus{},
		},
	}

	for i, m := range members {
		rack := fmt.Sprintf("rack-%d", i)
		c.Spec.Datacenter.Racks = append(c.Spec.Datacenter.Racks, scyllav1alpha1.RackSpec{
			Name:    rack,
			Members: pointer.Int32(m),
			Storage: scyllav1alpha1.StorageSpec{
				Capacity: "5Gi",
			},
		})
		c.Status.Racks[rack] = scyllav1alpha1.RackStatus{
			Image:        image,
			Members:      pointer.Int32(m),
			ReadyMembers: pointer.Int32(m),
			Stale:        pointer.Bool(false),
		}
	}

	return c
}
