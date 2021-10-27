package nodeconfigdaemon

import (
	"os"
	"path"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func makePerftuneJob(nc *scyllav1alpha1.NodeConfig, nodeName, image, ifaceName, irqMask string, dataHostPaths []string, disableWritebackCache bool) *batchv1.Job {
	args := []string{
		"--tune", "system", "--tune-clock",
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
		naming.NodeConfigNameLabel:       nc.Name,
		naming.NodeConfigControllerLabel: nodeName,
	}

	perftuneJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.PerftuneJobName(nodeName),
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(nc, controllerGVK),
			},
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Tolerations:   nc.Spec.Placement.Tolerations,
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
								volumeMount("hostfs", naming.HostFilesystemDirName, false),
								volumeMount("etc-systemd", "/etc/systemd", false),
								volumeMount("host-sys-class", "/sys/class", false),
								volumeMount("host-sys-devices", "/sys/devices", false),
								volumeMount("host-lib-systemd-system", "/lib/systemd/system", true),
								volumeMount("host-var-run-dbus", "/var/run/dbus", true),
								volumeMount("host-run-systemd-system", "/run/systemd/system", true),
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
						hostDirVolume("hostfs", "/"),
						hostDirVolume("etc-systemd", "/etc/systemd"),
						hostDirVolume("host-sys-class", "/sys/class"),
						hostDirVolume("host-sys-devices", "/sys/devices"),
						hostDirVolume("host-lib-systemd-system", "/lib/systemd/system"),
						hostDirVolume("host-var-run-dbus", "/var/run/dbus"),
						hostDirVolume("host-run-systemd-system", "/run/systemd/system"),
					},
				},
			},
		},
	}

	// Host node might not be running irqbalance. Mount config only when it's present on the host.
	_, err := os.Stat(path.Join(naming.HostFilesystemDirName, "/etc/sysconfig/irqbalance"))
	if err == nil {
		perftuneJob.Spec.Template.Spec.Volumes = append(perftuneJob.Spec.Template.Spec.Volumes,
			hostFileVolume("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance"),
		)
		perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts = append(perftuneJob.Spec.Template.Spec.Containers[0].VolumeMounts,
			volumeMount("etc-sysconfig-irqbalance", "/etc/sysconfig/irqbalance", false),
		)
	}

	return perftuneJob
}

func volumeMount(name, mountPath string, readonly bool) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
		ReadOnly:  readonly,
	}
}

func hostVolume(name, hostPath string, volumeType *corev1.HostPathType) corev1.Volume {
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

func hostDirVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathDirectory
	return hostVolume(name, hostPath, &volumeType)
}

func hostFileVolume(name, hostPath string) corev1.Volume {
	volumeType := corev1.HostPathFile
	return hostVolume(name, hostPath, &volumeType)
}
