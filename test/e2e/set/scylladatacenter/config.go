package scylladatacenter

import (
	"time"
)

const (
	updateFromScyllaImage  = "docker.io/scylladb/scylla:4.6.2"
	updateToScyllaImage    = "docker.io/scylladb/scylla:4.6.3"
	upgradeFromScyllaImage = "docker.io/scylladb/scylla:4.5.5"
	upgradeToScyllaImage   = "docker.io/scylladb/scylla:4.6.3"

	testTimeout = 45 * time.Minute
)
