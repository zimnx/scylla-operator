package metrics

import "github.com/prometheus/client_golang/prometheus"

const (
	ClusterKey = "cluster"
	HostKey    = "host"
	appKey     = "app"

	sidecarApp = "sidecar"
)

var (
	LivenessProbe = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "scylla_operator",
		Subsystem: "probes",
		Name:      "liveness_duration_seconds",
		Help:      "Histogram of the time (in seconds) each liveness probe took.",
		Buckets:   append([]float64{0.001, 0.0025}, prometheus.DefBuckets...),
	}, []string{ClusterKey, appKey, HostKey}).MustCurryWith(prometheus.Labels{
		appKey: sidecarApp,
	})

	ReadinessProbe = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "scylla_operator",
		Subsystem: "probes",
		Name:      "readiness_duration_seconds",
		Help:      "Histogram of the time (in seconds) each readiness probe took.",
		Buckets:   append([]float64{0.001, 0.0025}, prometheus.DefBuckets...),
	}, []string{ClusterKey, appKey, HostKey}).MustCurryWith(prometheus.Labels{
		appKey: sidecarApp,
	})
)

func init() {
	prometheus.MustRegister(
		LivenessProbe,
		ReadinessProbe,
	)
}
