// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"path"

	"github.com/c9s/goprocinfo/linux"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/semver"
	"github.com/scylladb/scylla-operator/pkg/util/cloud"
	"github.com/scylladb/scylla-operator/pkg/util/cpuset"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (ncdc *Controller) makePerftuneJobForContainers(ctx context.Context, snt *scyllav1alpha1.NodeConfig, optimizablePods []*corev1.Pod, podSpec *corev1.PodSpec) (*batchv1.Job, error) {
	if len(optimizablePods) == 0 {
		klog.V(4).InfoS("No optimizable pod found on this node")
		return nil, nil
	}

	hasher := sha512.New()
	for _, p := range optimizablePods {
		hasher.Write([]byte(fmt.Sprintf("%s-%s\n", p.Namespace, p.Name)))
	}
	jobName := fmt.Sprintf("perftune-%s", base64.URLEncoding.EncodeToString(hasher.Sum(nil)))

	cpuInfo, err := linux.ReadCPUInfo(path.Join(naming.HostFilesystemDirName, "/proc/cpuinfo"))
	if err != nil {
		return nil, fmt.Errorf("couldn't parse cpuinfo from %q: %w", "/proc/cpuinfo", err)
	}

	hostFullCpuset, err := cpuset.Parse(fmt.Sprintf("0-%d", cpuInfo.NumCPU()-1))
	if err != nil {
		return nil, fmt.Errorf("can't parse full mask: %w", err)
	}

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
		jobName,
		ncdc.nodeName,
		ncdc.scyllaImage,
		iface.Name,
		irqCPUs.FormatMask(),
		dataHostPaths,
		disableWritebackCache,
		podSpec,
	), nil
}

func (ncdc *Controller) makeJobsForContainers(ctx context.Context, snt *scyllav1alpha1.NodeConfig, scyllaPods []*corev1.Pod, existingJobs map[string]*batchv1.Job) ([]*batchv1.Job, error) {
	var optimizablePods []*corev1.Pod
	for i := range scyllaPods {
		pod := scyllaPods[i]

		if pod.Status.QOSClass != corev1.PodQOSGuaranteed {
			klog.V(4).Infof("Pod %s is not subject for optimizations", naming.ObjRef(pod))
			continue
		}

		if !helpers.IsScyllaContainerRunning(pod) {
			klog.V(4).Infof("Pod %s is a candidate for optimizations but scylla container isn't running yet", naming.ObjRef(pod))
		}

		klog.V(4).Infof("Pod %s is subject for optimizations", naming.ObjRef(pod))
		optimizablePods = append(optimizablePods, pod)
	}

	pod, err := ncdc.selfPodLister.Pods(ncdc.namespace).Get(ncdc.podName)
	if err != nil {
		return nil, fmt.Errorf("can't get Pod %s/%s: %w", ncdc.namespace, ncdc.podName, err)
	}

	var errs []error
	var jobs []*batchv1.Job

	perftuneJob, err := ncdc.makePerftuneJobForContainers(ctx, snt, optimizablePods, &pod.Spec)
	errs = append(errs, err)
	if perftuneJob != nil {
		jobs = append(jobs, perftuneJob)
	}

	return jobs, utilerrors.NewAggregate(errs)
}

func (ncdc *Controller) syncJobsForContainers(ctx context.Context, snt *scyllav1alpha1.NodeConfig, scyllaPods []*corev1.Pod, jobs map[string]*batchv1.Job) error {
	required, err := ncdc.makeJobsForContainers(ctx, snt, scyllaPods, jobs)
	if err != nil {
		return fmt.Errorf("can't make Jobs: %w", err)
	}
	err = ncdc.pruneJobs(ctx, jobs, required)
	if err != nil {
		return fmt.Errorf("can't delete Jobs: %w", err)
	}

	for _, j := range required {
		_, _, err := resourceapply.ApplyJob(ctx, ncdc.kubeClient.BatchV1(), ncdc.namespacedJobLister, ncdc.eventRecorder, j)
		if err != nil {
			return fmt.Errorf("can't create job %s: %w", naming.ObjRef(j), err)
		}
	}
	return nil
}
