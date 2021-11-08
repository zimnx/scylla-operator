package nodeconfigdaemon

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"

	"github.com/scylladb/scylla-operator/pkg/naming"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// TODO: set anti affinities so config jobs don't run on the same node at the same time

func makePerftuneJobForNode(controllerRef *metav1.OwnerReference, namespace, nodeName, image string, podSpec *corev1.PodSpec) *batchv1.Job {
	podSpec = podSpec.DeepCopy()

	args := []string{
		"--tune=system",
		"--tune-clock",
		"--tune=net",
	}

	labels := map[string]string{
		naming.NodeConfigNameLabel:       controllerRef.Name,
		naming.NodeConfigJobForNodeLabel: nodeName,
		naming.NodeConfigJobTypeLabel:    string(naming.NodeConfigJobTypeNode),
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			// TODO: hash the name to avoid overflow.
			Name:            fmt.Sprintf("perftune-%s-node", nodeName),
			OwnerReferences: []metav1.OwnerReference{*controllerRef},
			Labels:          labels,
		},
		Spec: batchv1.JobSpec{
			// TODO: handle failed jobs and retry.
			BackoffLimit: pointer.Int32Ptr(math.MaxInt32),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Tolerations:   podSpec.Tolerations,
					NodeName:      nodeName,
					RestartPolicy: corev1.RestartPolicyOnFailure,
					HostPID:       true,
					HostNetwork:   true,
					Containers: []corev1.Container{
						{
							Name:            naming.PerftuneContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/opt/scylladb/scripts/perftune.py"},
							Args:            args,
							Env: []corev1.EnvVar{
								{
									Name:  "SYSTEMD_IGNORE_CHROOT",
									Value: "1",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								makeVolumeMount("hostfs", naming.HostFilesystemDirName, false),
								makeVolumeMount("etc-systemd", "/etc/systemd", false),
								makeVolumeMount("host-sys-class", "/sys/class", false),
								makeVolumeMount("host-sys-devices", "/sys/devices", false),
								makeVolumeMount("host-lib-systemd-system", "/lib/systemd/system", true),
								makeVolumeMount("host-var-run-dbus", "/var/run/dbus", true),
								makeVolumeMount("host-run-systemd-system", "/run/systemd/system", true),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						// FIXME: revisit which ones are actually needed
						makeHostDirVolume("hostfs", "/"),
						makeHostDirVolume("etc-systemd", "/etc/systemd"),
						makeHostDirVolume("host-sys-class", "/sys/class"),
						makeHostDirVolume("host-sys-devices", "/sys/devices"),
						makeHostDirVolume("host-lib-systemd-system", "/lib/systemd/system"),
						makeHostDirVolume("host-var-run-dbus", "/var/run/dbus"),
						makeHostDirVolume("host-run-systemd-system", "/run/systemd/system"),
					},
				},
			},
		},
	}

	return job
}

type perftuneJobForContainersData struct {
	ContainerIDs []string `json:"containerIDs"`
}

func makePerftuneJobForContainers(controllerRef *metav1.OwnerReference, namespace, nodeName, image, ifaceName, irqMask string, dataHostPaths []string, disableWritebackCache bool, podSpec *corev1.PodSpec, scyllaContainerIDs []string) (*batchv1.Job, error) {
	podSpec = podSpec.DeepCopy()

	args := []string{
		"--tune", "net", "--nic", ifaceName, "--irq-cpu-mask", irqMask,
	}

	// FIXME: disk shouldn't be empty
	if len(dataHostPaths) > 0 {
		args = append(args, "--tune", "disks")
	}
	for _, hostPath := range dataHostPaths {
		args = append(args, "--dir", path.Join(naming.HostFilesystemDirName, hostPath))
	}

	if disableWritebackCache {
		args = append(args, "--write-back-cache", "false")
	}

	labels := map[string]string{
		naming.NodeConfigNameLabel:       controllerRef.Name,
		naming.NodeConfigJobForNodeLabel: nodeName,
		naming.NodeConfigJobTypeLabel:    string(naming.NodeConfigJobTypeContainers),
	}

	jobData := perftuneJobForContainersData{
		ContainerIDs: scyllaContainerIDs,
	}
	jobDataBytes, err := json.Marshal(jobData)
	if err != nil {
		return nil, fmt.Errorf("can't marshal job data: %w", err)
	}

	perftuneJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			// TODO: hash the name to avoid overflow
			Name:            fmt.Sprintf("perftune-%s-containers", nodeName),
			OwnerReferences: []metav1.OwnerReference{*controllerRef},
			Labels:          labels,
			Annotations: map[string]string{
				naming.NodeConfigJobData: string(jobDataBytes),
			},
		},
		Spec: batchv1.JobSpec{
			// TODO: handle failed jobs and retry.
			BackoffLimit: pointer.Int32Ptr(math.MaxInt32),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Tolerations:   podSpec.Tolerations,
					NodeName:      nodeName,
					RestartPolicy: corev1.RestartPolicyOnFailure,
					HostPID:       true,
					HostNetwork:   true,
					Containers: []corev1.Container{
						{
							Name:            naming.PerftuneContainerName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/opt/scylladb/scripts/perftune.py"},
							Args:            args,
							Env: []corev1.EnvVar{
								{
									Name:  "SYSTEMD_IGNORE_CHROOT",
									Value: "1",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.BoolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								makeVolumeMount("hostfs", naming.HostFilesystemDirName, false),
								makeVolumeMount("etc-systemd", "/etc/systemd", false),
								makeVolumeMount("host-sys-class", "/sys/class", false),
								makeVolumeMount("host-sys-devices", "/sys/devices", false),
								makeVolumeMount("host-lib-systemd-system", "/lib/systemd/system", true),
								makeVolumeMount("host-var-run-dbus", "/var/run/dbus", true),
								makeVolumeMount("host-run-systemd-system", "/run/systemd/system", true),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						// FIXME: revisit which ones are actually needed
						makeHostDirVolume("hostfs", "/"),
						makeHostDirVolume("etc-systemd", "/etc/systemd"),
						makeHostDirVolume("host-sys-class", "/sys/class"),
						makeHostDirVolume("host-sys-devices", "/sys/devices"),
						makeHostDirVolume("host-lib-systemd-system", "/lib/systemd/system"),
						makeHostDirVolume("host-var-run-dbus", "/var/run/dbus"),
						makeHostDirVolume("host-run-systemd-system", "/run/systemd/system"),
					},
				},
			},
		},
	}

	// Host node might not be running irqbalance. Mount config only when it's present on the host.
	_, err = os.Stat(path.Join(naming.HostFilesystemDirName, "/etc/sysconfig/irqbalance"))
	if err == nil {
		perftuneJob.Spec.Template.Spec.Volumes = append(
			perftuneJob.Spec.Template.Spec.Volumes,
			makeHostFileVolume("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance"),
		)
		perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts,
			makeVolumeMount("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance", false),
		)
	}

	return perftuneJob, nil
}

func makeVolumeMount(name, mountPath string, readonly bool) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
		ReadOnly:  readonly,
	}
}

func makeHostVolume(name, hostPath string, volumeType *corev1.HostPathType) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: hostPath,
				Type: volumeType,
			},
		},
	}
}

func makeHostDirVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathDirectory
	return makeHostVolume(name, hostPath, &volumeType)
}

func makeHostFileVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathFile
	return makeHostVolume(name, hostPath, &volumeType)
}
