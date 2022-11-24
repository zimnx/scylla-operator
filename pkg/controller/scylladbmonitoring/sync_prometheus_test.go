package scylladbmonitoring

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeScyllaDBServiceMonitor(t *testing.T) {
	tt := []struct {
		name           string
		sm             *scyllav1alpha1.ScyllaDBMonitoring
		expectedString string
		expectedErr    error
	}{
		{
			name: "empty selector",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: "sm-name-scylladb"
spec:
  selector:
    {}
  endpoints:
  - port: prometheus
    honorLabels: false
    metricRelabelings:
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CPU
      replacement: 'cpu'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CQL
      replacement: 'cql'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: OS
      replacement: 'os'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: IO
      replacement: 'io'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: Errors
      replacement: 'errors'
    - regex: 'help|exported_instance'
      action: labeldrop
    - sourceLabels: [version]
      regex: '([0-9]+\.[0-9]+)(\.?[0-9]*).*'
      replacement: '$1$2'
      targetLabel: svr
    relabelings:
    - sourceLabels: [__address__]
      regex:  '(.*):.+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
  - port: web
    honorLabels: false
    relabelings:
    - sourceLabels: [__address__]
      regex:  '(.*):.+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
    metricRelabelings:
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CPU
      replacement: 'cpu'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CQL
      replacement: 'cql'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: OS
      replacement: 'os'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: IO
      replacement: 'io'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: Errors
      replacement: 'errors'
    - regex: 'help|exported_instance'
      action: labeldrop
    - sourceLabels: [version]
      regex: '([0-9]+\.[0-9]+)(\.?[0-9]*).*'
      replacement: '$1$2'
      targetLabel: svr
`, "\n"),
			expectedErr: nil,
		},
		{
			name: "specific selector",
			sm: &scyllav1alpha1.ScyllaDBMonitoring{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sm-name",
				},
				Spec: scyllav1alpha1.ScyllaDBMonitoringSpec{
					EndpointsSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "alpha",
								Operator: metav1.LabelSelectorOpExists,
								Values:   []string{"beta"},
							},
						},
					},
				},
			},
			expectedString: strings.TrimLeft(`
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: "sm-name-scylladb"
spec:
  selector:
    matchExpressions:
    - key: alpha
      operator: Exists
      values:
      - beta
    matchLabels:
      foo: bar
  endpoints:
  - port: prometheus
    honorLabels: false
    metricRelabelings:
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CPU
      replacement: 'cpu'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CQL
      replacement: 'cql'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: OS
      replacement: 'os'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: IO
      replacement: 'io'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: Errors
      replacement: 'errors'
    - regex: 'help|exported_instance'
      action: labeldrop
    - sourceLabels: [version]
      regex: '([0-9]+\.[0-9]+)(\.?[0-9]*).*'
      replacement: '$1$2'
      targetLabel: svr
    relabelings:
    - sourceLabels: [__address__]
      regex:  '(.*):.+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
  - port: web
    honorLabels: false
    relabelings:
    - sourceLabels: [__address__]
      regex:  '(.*):.+'
      targetLabel: instance
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_service_label_scylla_cluster]
      regex:  '(.+)'
      targetLabel: cluster
      replacement: '${1}'
    - sourceLabels: [__meta_kubernetes_pod_label_scylla_datacenter]
      regex:  '(.+)'
      targetLabel: dc
      replacement: '${1}'
    metricRelabelings:
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CPU
      replacement: 'cpu'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: CQL
      replacement: 'cql'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: OS
      replacement: 'os'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: IO
      replacement: 'io'
    - sourceLabels: [version]
      regex:  '(.+)'
      targetLabel: Errors
      replacement: 'errors'
    - regex: 'help|exported_instance'
      action: labeldrop
    - sourceLabels: [version]
      regex: '([0-9]+\.[0-9]+)(\.?[0-9]*).*'
      replacement: '$1$2'
      targetLabel: svr
`, "\n"),
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, objString, err := makeScyllaDBServiceMonitor(tc.sm)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected and got errors differ:\n%s\nRendered object:\n%s", cmp.Diff(tc.expectedErr, err), objString)
			}

			if objString != tc.expectedString {
				t.Errorf("expected and got strins differ:\n%s", cmp.Diff(
					[]byte(tc.expectedString),
					[]byte(objString),
				))
			}
		})
	}
}
