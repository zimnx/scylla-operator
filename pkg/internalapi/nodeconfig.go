package internalapi

type SidecarRuntimeConfig struct {
	// ContainerID hold the ID of the scylla container this information is valid for.
	// E.g. on restarts, the container gets a new ID.
	ContainerID string `json:"containerID"`

	// BlockingNodeConfigs is a list of NodeConfigs this pod is waiting on.
	BlockingNodeConfigs []string `json:"blockingNodeConfigs"`
}
