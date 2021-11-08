// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/c9s/goprocinfo/linux"
	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/util/cloud"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func (ncdc *Controller) makePerftuneJobForContainers(ctx context.Context, podSpec *corev1.PodSpec, optimizablePods []*corev1.Pod, scyllaContainerIDs []string) (*batchv1.Job, error) {
	cpuInfo, err := linux.ReadCPUInfo(path.Join(naming.HostFilesystemDirName, "/proc/cpuinfo"))
	if err != nil {
		return nil, fmt.Errorf("can't parse cpuinfo from %q: %w", "/proc/cpuinfo", err)
	}

	hostFullCpuset, err := cpuset.Parse(fmt.Sprintf("0-%d", cpuInfo.NumCPU()-1))
	if err != nil {
		return nil, fmt.Errorf("can't parse full mask: %w", err)
	}

	// TODO: see if we can plumb in the container IDs we already have.

	irqCPUs, err := getIRQCPUs(ctx, ncdc.criClient, optimizablePods, hostFullCpuset)
	if err != nil {
		return nil, fmt.Errorf("can't get IRQ CPUs: %w", err)
	}

	dataHostPaths, err := scyllaDataDirHostPaths(ctx, ncdc.criClient, optimizablePods)
	if err != nil {
		return nil, fmt.Errorf("can't find data dir host path: %w", err)
	}

	iface, err := network.FindEthernetInterface()
	if err != nil {
		return nil, fmt.Errorf("can't find local interface")
	}

	disableWritebackCache := false
	if cloud.OnGKE() {
		scyllaVersion, err := naming.ImageToVersion(ncdc.scyllaImage)
		if err != nil {
			return nil, fmt.Errorf("can't determine scylla image version %q: %w", ncdc.scyllaImage, err)
		}
		sv := semver.NewScyllaVersion(scyllaVersion)

		if sv.SupportFeatureSafe(semver.ScyllaVersionThatSupportsDisablingWritebackCache) {
			disableWritebackCache = true
		}
	}

	return makePerftuneJobForContainers(
		ncdc.newControllerRef(),
		ncdc.namespace,
		ncdc.nodeName,
		ncdc.scyllaImage,
		iface.Name,
		irqCPUs.FormatMask(),
		dataHostPaths,
		disableWritebackCache,
		podSpec,
		scyllaContainerIDs,
	)
}

func (ncdc *Controller) makeJobForContainers(ctx context.Context) (*batchv1.Job, error) {
	localScyllaPods, err := ncdc.localScyllaPodsLister.List(naming.ScyllaSelector())
	if err != nil {
		return nil, fmt.Errorf("can't list local scylla pods: %w", err)
	}

	var optimizablePods []*corev1.Pod
	var scyllaContainerIDs []string
	for i := range localScyllaPods {
		scyllaPod := localScyllaPods[i]

		if scyllaPod.Status.QOSClass != corev1.PodQOSGuaranteed {
			klog.V(4).Infof("Pod %q isn't a subject for optimizations", naming.ObjRef(scyllaPod))
			continue
		}

		if !helpers.IsScyllaContainerRunning(scyllaPod) {
			klog.V(4).Infof("Pod %q is a candidate for optimizations but scylla container isn't running yet", naming.ObjRef(scyllaPod))
		}

		klog.V(4).Infof("Pod %s is subject for optimizations", naming.ObjRef(scyllaPod))
		optimizablePods = append(optimizablePods, scyllaPod)

		for _, cs := range scyllaPod.Status.ContainerStatuses {
			if cs.Name == naming.ScyllaContainerName {
				if len(cs.ContainerID) == 0 {
					ncdc.eventRecorder.Event(ncdc.newObjectRef(), corev1.EventTypeWarning, "MissingContainerID", "Scylla container status is missing a containerID. Scylla won't wait for tuning to finish.")
					continue
				}

				scyllaContainerIDs = append(scyllaContainerIDs, cs.ContainerID)
			}
		}
	}

	if len(optimizablePods) == 0 {
		klog.V(2).InfoS("No optimizable pod found on this node")
		return nil, nil
	}

	selfPod, err := ncdc.selfPodLister.Pods(ncdc.namespace).Get(ncdc.podName)
	if err != nil {
		return nil, fmt.Errorf("can't get Pod %q: %w", naming.ManualRef(ncdc.namespace, ncdc.podName), err)
	}

	return ncdc.makePerftuneJobForContainers(ctx, &selfPod.Spec, optimizablePods, scyllaContainerIDs)
}

func (ncdc *Controller) syncPertuneJobForContainers(ctx context.Context, existingJobs map[string]*batchv1.Job, nodeStatus *v1alpha1.NodeStatus) error {
	required, err := ncdc.makeJobForContainers(ctx)
	if err != nil {
		return fmt.Errorf("can't make preftune Job for containers: %w", err)
	}

	err = ncdc.pruneJobs(ctx, existingJobs, []*batchv1.Job{required})
	if err != nil {
		return fmt.Errorf("can't prune perftune Jobs: %w", err)
	}

	if required != nil {
		fresh, _, err := resourceapply.ApplyJob(ctx, ncdc.kubeClient.BatchV1(), ncdc.namespacedJobLister, ncdc.eventRecorder, required)
		if err != nil {
			return fmt.Errorf("can't apply job %q: %w", naming.ObjRef(required), err)
		}

		// We have successfully applied the job definition so the data should always be present at this point.
		nodeConfigJobDataString, found := fresh.Annotations[naming.NodeConfigJobData]
		if !found {
			return fmt.Errorf("internal error: job %q is missing %q annotation", klog.KObj(fresh), naming.NodeConfigJobData)
		}

		jobData := &perftuneJobForContainersData{}
		err = json.Unmarshal([]byte(nodeConfigJobDataString), jobData)
		if err != nil {
			return fmt.Errorf("internal error: can't unmarshal node config data for job %q: %w", klog.KObj(fresh), err)
		}
		nodeStatus.TunedContainers = jobData.ContainerIDs
	}

	return nil
}