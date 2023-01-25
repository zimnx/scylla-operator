// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeScyllaOperatorNodeTuningNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
	}
}

func makeNodeConfigServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.NodeConfigAppName,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
	}
}

func NodeConfigClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.NodeConfigAppName,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"scylla.scylladb.com"},
				Resources: []string{"nodeconfigs"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"scylla.scylladb.com"},
				Resources: []string{"nodeconfigs/status"},
				Verbs:     []string{"update"},
			},
		},
	}
}

func makeNodeConfigClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.NodeConfigAppName,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels: map[string]string{
				naming.NodeConfigNameLabel: naming.NodeConfigAppName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     naming.NodeConfigAppName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: naming.ScyllaOperatorNodeTuningNamespace,
				Name:      naming.NodeConfigAppName,
			},
		},
	}
}

func makeNodeDiskSetupDaemonSet(nc *scyllav1alpha1.NodeConfig, operatorImage string) *appsv1.DaemonSet {
	if nc.Spec.LocalDiskSetup == nil {
		return nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       naming.NodeConfigAppName,
		naming.NodeConfigNameLabel:     nc.Name,
		naming.NodeConfigDaemonSetType: naming.NodeConfigDaemonSetTypeNodeSetup,
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-node-setup", nc.Name),
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(nc, nodeConfigControllerGVK),
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: naming.NodeConfigAppName,
					NodeSelector:       nc.Spec.Placement.NodeSelector,
					Affinity:           &nc.Spec.Placement.Affinity,
					Tolerations:        nc.Spec.Placement.Tolerations,
					Volumes: []corev1.Volume{
						{
							Name: "hostfs",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
									Type: pointer.Ptr(corev1.HostPathDirectory),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  naming.NodeConfigAppName,
							Image: operatorImage,
							// FIXME:
							ImagePullPolicy: corev1.PullAlways,
							Command: []string{
								"/usr/bin/bash",
								"-euExo",
								"pipefail",
								"-c",
							},
							Args: []string{
								`
cd "$( mktemp -d )"

for f in $( find /host -mindepth 1 -maxdepth 1 -type d -printf '%f\n' ); do
	mkdir -p "./${f}"
	mount --rbind "/host/${f}" "./${f}"
done

for f in $( find /host -mindepth 1 -maxdepth 1 -type f -printf '%f\n' ); do
	touch "./${f}"
	mount --bind "/host/${f}" "./${f}"
done

find /host -mindepth 1 -maxdepth 1 -type l -exec cp -P "{}" ./ \;

mkdir -p ./scylla-operator
touch ./scylla-operator/scylla-operator
mount --bind /usr/bin/scylla-operator ./scylla-operator/scylla-operator

for f in ca.crt token; do
	touch "./scylla-operator/${f}"
	mount --bind "/var/run/secrets/kubernetes.io/serviceaccount/${f}" "./scylla-operator/${f}"
done

cat <<EOF > ./scylla-operator/kubeconfig
apiVersion: v1
kind: Config
clusters:
- name: in-cluster
  cluster:
    server: https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}
    certificate-authority: /scylla-operator/ca.crt

users:
- name: scylla-operator
  user:
    tokenFile: /scylla-operator/token

contexts:
- name: in-cluster
  context:
    cluster: in-cluster
    user: scylla-operator

current-context: in-cluster

EOF

exec chroot ./ /scylla-operator/scylla-operator node-setup-daemon \
--kubeconfig=/scylla-operator/kubeconfig \
--namespace="$(NAMESPACE)" \
--node-name="$(NODE_NAME)" \
--node-config-name=` + fmt.Sprintf("%q", nc.Name) + ` \
--node-config-uid=` + fmt.Sprintf("%q", nc.UID) + ` \
--loglevel=` + fmt.Sprintf("%d", 4) + `
							`},
							Env: []corev1.EnvVar{
								{
									Name:  "SYSTEMD_IGNORE_CHROOT",
									Value: "1",
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Ptr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:             "hostfs",
									MountPath:        "/host",
									MountPropagation: pointer.Ptr(corev1.MountPropagationBidirectional),
								},
							},
						},
					},
				},
			},
		},
	}
}

func makeNodeConfigDaemonSet(nc *scyllav1alpha1.NodeConfig, operatorImage, scyllaImage string) *appsv1.DaemonSet {
	if nc.Spec.DisableOptimizations {
		return nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       naming.NodeConfigAppName,
		naming.NodeConfigNameLabel:     nc.Name,
		naming.NodeConfigDaemonSetType: naming.NodeConfigDaemonSetTypeTuning,
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nc.Name,
			Namespace: naming.ScyllaOperatorNodeTuningNamespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(nc, nodeConfigControllerGVK),
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: naming.NodeConfigAppName,
					// Required for getting the right iface name to tune
					HostNetwork:  true,
					NodeSelector: nc.Spec.Placement.NodeSelector,
					Affinity:     &nc.Spec.Placement.Affinity,
					Tolerations:  nc.Spec.Placement.Tolerations,
					Volumes: []corev1.Volume{
						makeHostDirVolume("hostfs", "/"),
					},
					Containers: []corev1.Container{
						{
							Name:            naming.NodeConfigAppName,
							Image:           operatorImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/bash",
								"-euExo",
								"pipefail",
								"-c",
							},
							Args: []string{
								`
shopt -s inherit_errexit

cd "$( mktemp -d )"

for f in $( find /host -mindepth 1 -maxdepth 1 -type d -printf '%f\n' ); do
	mkdir -p "./${f}"
	mount --rbind "/host/${f}" "./${f}"
done

for f in $( find /host -mindepth 1 -maxdepth 1 -type f -printf '%f\n' ); do
	touch "./${f}"
	mount --bind "/host/${f}" "./${f}"
done

find /host -mindepth 1 -maxdepth 1 -type l -exec cp -P "{}" ./ \;

mkdir -p ./scylla-operator
touch ./scylla-operator/scylla-operator
mount --bind /usr/bin/scylla-operator ./scylla-operator/scylla-operator

for f in ca.crt token; do
	touch "./scylla-operator/${f}"
	mount --bind "/var/run/secrets/kubernetes.io/serviceaccount/${f}" "./scylla-operator/${f}"
done

cat <<EOF > ./scylla-operator/kubeconfig
apiVersion: v1
kind: Config
clusters:
- name: in-cluster
  cluster:
    server: https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}
    certificate-authority: /scylla-operator/ca.crt

users:
- name: scylla-operator
  user:
    tokenFile: /scylla-operator/token

contexts:
- name: in-cluster
  context:
    cluster: in-cluster
    user: scylla-operator

current-context: in-cluster

EOF

exec chroot ./ /scylla-operator/scylla-operator node-config-daemon \
--kubeconfig=/scylla-operator/kubeconfig \
--pod-name="$(POD_NAME)" \
--namespace="$(POD_NAMESPACE)" \
--node-name="$(NODE_NAME)" \
--node-config-name=` + fmt.Sprintf("%q", nc.Name) + ` \
--node-config-uid=` + fmt.Sprintf("%q", nc.UID) + ` \
--scylla-image=` + fmt.Sprintf("%q", scyllaImage) + ` \
--disable-optimizations=` + fmt.Sprintf("%t", nc.Spec.DisableOptimizations) + ` \
--loglevel=` + fmt.Sprintf("%d", 4) + `
							`,
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Ptr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								makeVolumeMount("hostfs", "/host", false),
							},
						},
					},
				},
			},
		},
	}
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
