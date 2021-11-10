package controllerhelpers

import (
	"reflect"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsNodeConfigSelectingNode(t *testing.T) {
	tt := []struct {
		name        string
		placement   *scyllav1alpha1.NodeConfigPlacement
		nodeLabels  map[string]string
		nodeTaints  []corev1.Taint
		expected    bool
		expectedErr error
	}{
		{
			name:      "empty placement selects non-tained node",
			placement: &scyllav1alpha1.NodeConfigPlacement{},
			nodeLabels: map[string]string{
				"foo": "bar",
			},
			nodeTaints:  nil,
			expected:    true,
			expectedErr: nil,
		},
		{
			name: "node selector won't match a node without the label",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				NodeSelector: map[string]string{
					"alpha": "beta",
				},
			},
			nodeLabels: map[string]string{
				"foo": "bar",
			},
			nodeTaints:  nil,
			expected:    false,
			expectedErr: nil,
		},
		{
			name: "node selector will match a node with the label",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				NodeSelector: map[string]string{
					"alpha": "beta",
				},
			},
			nodeLabels: map[string]string{
				"alpha": "beta",
			},
			nodeTaints:  nil,
			expected:    true,
			expectedErr: nil,
		},
		{
			name:       "placement without any toleration won't select tained node",
			placement:  &scyllav1alpha1.NodeConfigPlacement{},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expected:    false,
			expectedErr: nil,
		},
		{
			name:       "placement without any toleration will select tained node with effects other then NoSchedule and NoExecute",
			placement:  &scyllav1alpha1.NodeConfigPlacement{},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
			},
			expected:    true,
			expectedErr: nil,
		},
		{
			name: "placement without matching toleration won't select tained node",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				Tolerations: []corev1.Toleration{
					{
						Key:      "alpha",
						Value:    "beta",
						Effect:   corev1.TaintEffectNoSchedule,
						Operator: corev1.TolerationOpEqual,
					},
				},
			},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expected:    false,
			expectedErr: nil,
		},
		{
			name: "placement with matching toleration will select tained node",
			placement: &scyllav1alpha1.NodeConfigPlacement{
				Tolerations: []corev1.Toleration{
					{
						Key:      "foo",
						Value:    "bar",
						Operator: corev1.TolerationOpEqual,
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			},
			nodeLabels: nil,
			nodeTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expected:    true,
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			nc := &scyllav1alpha1.NodeConfig{
				Spec: scyllav1alpha1.NodeConfigSpec{
					Placement: *tc.placement,
				},
			}

			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.nodeLabels,
				},
				Spec: corev1.NodeSpec{
					Taints: tc.nodeTaints,
				},
			}

			got, err := IsNodeConfigSelectingNode(nc, node)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
			}

			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
