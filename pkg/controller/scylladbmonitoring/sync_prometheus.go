package scylladbmonitoring

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"time"

	prometheusv1assets "github.com/scylladb/scylla-operator/assets/monitoring/prometheus/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	okubecrypto "github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/pointer"
)

func getPrometheusLabels(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Set {
	return helpers.MergeMaps(
		getLabels(sm),
		labels.Set{
			naming.ControllerNameLabel: "prometheus",
		},
	)
}

func getPrometheusSelector(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Selector {
	return labels.SelectorFromSet(getPrometheusLabels(sm))
}

func makeScyllaDBServiceMonitor(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.ServiceMonitor, string, error) {
	return prometheusv1assets.ScyllaDBServiceMonitorTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"endpointsSelector":      sm.Spec.EndpointsSelector,
	})
}

func makePrometheusRule(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.PrometheusRule, string, error) {
	return prometheusv1assets.PrometheusRuleTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makePrometheus(sm *scyllav1alpha1.ScyllaDBMonitoring) (*monitoringv1.Prometheus, string, error) {
	var volumeClaimTemplate *monitoringv1.EmbeddedPersistentVolumeClaim
	if sm.Spec.Components != nil && sm.Spec.Components.Prometheus != nil && sm.Spec.Components.Prometheus.Storage != nil {
		volumeClaimTemplate = &monitoringv1.EmbeddedPersistentVolumeClaim{
			EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{
				Name:        fmt.Sprintf("%s-prometheus", sm.Name),
				Labels:      sm.Spec.Components.Prometheus.Storage.VolumeClaimTemplate.Labels,
				Annotations: sm.Spec.Components.Prometheus.Storage.VolumeClaimTemplate.Annotations,
			},
			Spec: sm.Spec.Components.Prometheus.Storage.VolumeClaimTemplate.Spec,
		}

	}

	affinity := corev1.Affinity{}
	var tolerations []corev1.Toleration
	if sm.Spec.Components != nil && sm.Spec.Components.Prometheus != nil && sm.Spec.Components.Prometheus.Placement != nil {
		affinity.NodeAffinity = sm.Spec.Components.Prometheus.Placement.NodeAffinity
		affinity.PodAffinity = sm.Spec.Components.Prometheus.Placement.PodAffinity
		affinity.PodAntiAffinity = sm.Spec.Components.Prometheus.Placement.PodAntiAffinity

		tolerations = sm.Spec.Components.Prometheus.Placement.Tolerations
	}

	var resources corev1.ResourceRequirements
	if sm.Spec.Components != nil && sm.Spec.Components.Prometheus != nil {
		resources = sm.Spec.Components.Prometheus.Resources
	}

	return prometheusv1assets.PrometheusTemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
		"volumeClaimTemplate":    volumeClaimTemplate,
		"affinity":               affinity,
		"tolerations":            tolerations,
		"resources":              resources,
	})
}

func (smc *Controller) syncPrometheus(
	ctx context.Context,
	sm *scyllav1alpha1.ScyllaDBMonitoring,
	prometheuses map[string]*monitoringv1.Prometheus,
	prometheusRules map[string]*monitoringv1.PrometheusRule,
	serviceMonitors map[string]*monitoringv1.ServiceMonitor,
	secrets map[string]*corev1.Secret,
	configMaps map[string]*corev1.ConfigMap,
	serviceAccounts map[string]*corev1.ServiceAccount,
	services map[string]*corev1.Service,
	roleBindings map[string]*rbacv1.RoleBinding,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	prometheusServingCertChainConfig := &okubecrypto.CertChainConfig{
		CAConfig: &okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-serving-ca", sm.Name),
				Labels: getPrometheusLabels(sm),
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		CABundleConfig: &okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-serving-ca", sm.Name),
				Labels: getPrometheusLabels(sm),
			},
		},
		CertConfigs: []*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   fmt.Sprintf("%s-prometheus-serving-certs", sm.Name),
					Labels: getPrometheusLabels(sm),
				},
				Validity: 30 * 24 * time.Hour,
				Refresh:  20 * 24 * time.Hour,
				CertCreator: (&ocrypto.ServingCertCreatorConfig{
					Subject: pkix.Name{
						CommonName: "",
					},
					IPAddresses: nil,
					DNSNames: []string{
						fmt.Sprintf("%s-prometheus", sm.Name),
						fmt.Sprintf("%s-prometheus.%s.svc", sm.Name, sm.Namespace),
					},
				}).ToCreator(),
			},
		},
	}

	prometheusClientCertChainConfig := &okubecrypto.CertChainConfig{
		CAConfig: &okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-client-ca", sm.Name),
				Labels: getPrometheusLabels(sm),
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		CABundleConfig: &okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-prometheus-client-ca", sm.Name),
				Labels: getPrometheusLabels(sm),
			},
		},
		CertConfigs: []*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   fmt.Sprintf("%s-prometheus-client-grafana", sm.Name),
					Labels: getPrometheusLabels(sm),
				},
				Validity: 10 * 365 * 24 * time.Hour,
				Refresh:  8 * 365 * 24 * time.Hour,
				CertCreator: (&ocrypto.ClientCertCreatorConfig{
					Subject: pkix.Name{
						CommonName: "",
					},
					DNSNames: []string{"grafana"},
				}).ToCreator(),
			},
		},
	}

	certChainConfigs := okubecrypto.CertChainConfigs{
		prometheusServingCertChainConfig,
		prometheusClientCertChainConfig,
	}

	// Render manifests.
	var renderErrors []error

	requiredPrometheusSA, _, err := prometheusv1assets.PrometheusSATemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
	renderErrors = append(renderErrors, err)

	requiredPrometheusRoleBinding, _, err := prometheusv1assets.PrometheusRoleBindingTemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
	renderErrors = append(renderErrors, err)

	requiredPrometheusService, _, err := prometheusv1assets.PrometheusServiceTemplate.RenderObject(map[string]any{
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
	renderErrors = append(renderErrors, err)

	requiredPrometheus, _, err := makePrometheus(sm)
	renderErrors = append(renderErrors, err)

	requiredPrometheusRule, _, err := makePrometheusRule(sm)
	renderErrors = append(renderErrors, err)

	requiredScyllaDBServiceMonitor, _, err := makeScyllaDBServiceMonitor(sm)
	renderErrors = append(renderErrors, err)

	renderError := kutilerrors.NewAggregate(renderErrors)
	if renderError != nil {
		return progressingConditions, renderError
	}

	// Prune objects.
	var pruneErrors []error
	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredPrometheusSA),
		serviceAccounts,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredPrometheusService),
		services,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Services(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredPrometheusRoleBinding),
		roleBindings,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredPrometheus),
		prometheuses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.Prometheuses(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredPrometheusRule),
		prometheusRules,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.PrometheusRules(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredScyllaDBServiceMonitor),
		serviceMonitors,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.ServiceMonitors(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		certChainConfigs.GetMetaSecrets(),
		secrets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Secrets(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		certChainConfigs.GetMetaConfigMaps(),
		configMaps,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	pruneError := kutilerrors.NewAggregate(pruneErrors)
	if pruneError != nil {
		return progressingConditions, pruneError
	}

	// Apply required objects.
	var applyErrors []error

	for _, item := range []struct {
		required kubeinterfaces.ObjectInterface
		control  resourceapply.ApplyControlUntypedInterface
	}{
		{
			required: requiredPrometheusSA,
			control: resourceapply.ApplyControlFuncs[*corev1.ServiceAccount]{
				GetCachedFunc: smc.serviceAccountLister.ServiceAccounts(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredPrometheusService,
			control: resourceapply.ApplyControlFuncs[*corev1.Service]{
				GetCachedFunc: smc.serviceLister.Services(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredPrometheusRoleBinding,
			control: resourceapply.ApplyControlFuncs[*rbacv1.RoleBinding]{
				GetCachedFunc: smc.roleBindingLister.RoleBindings(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredPrometheus,
			control: resourceapply.ApplyControlFuncs[*monitoringv1.Prometheus]{
				GetCachedFunc: smc.prometheusLister.Prometheuses(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.Prometheuses(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredScyllaDBServiceMonitor,
			control: resourceapply.ApplyControlFuncs[*monitoringv1.ServiceMonitor]{
				GetCachedFunc: smc.serviceMonitorLister.ServiceMonitors(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.ServiceMonitors(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredPrometheusRule,
			control: resourceapply.ApplyControlFuncs[*monitoringv1.PrometheusRule]{
				GetCachedFunc: smc.prometheusRuleLister.PrometheusRules(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.PrometheusRules(sm.Namespace).Update,
			}.ToUntyped(),
		},
	} {
		// Enforce namespace.
		item.required.SetNamespace(sm.Namespace)

		// Enforce labels for selection.
		if item.required.GetLabels() == nil {
			item.required.SetLabels(getPrometheusLabels(sm))
		} else {
			resourcemerge.MergeMapInPlaceWithoutRemovalKeys2(item.required.GetLabels(), getPrometheusLabels(sm))
		}

		// Set ControllerRef.
		item.required.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion:         scylladbMonitoringControllerGVK.GroupVersion().String(),
				Kind:               scylladbMonitoringControllerGVK.Kind,
				Name:               sm.Name,
				UID:                sm.UID,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			},
		})

		// Apply required object.
		_, changed, err := resourceapply.Apply(ctx, item.required, item.control, smc.eventRecorder, resourceapply.ApplyOptions{})
		if changed {
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, prometheusControllerProgressingCondition, item.required, "apply", sm.Generation)
		}
		if err != nil {
			gvk := resource.GetObjectGVKOrUnknown(item.required)
			applyErrors = append(applyErrors, fmt.Errorf("can't apply %s: %w", gvk, err))
		}
	}

	cm := okubecrypto.NewCertificateManager(
		smc.kubeClient.CoreV1(),
		smc.secretLister,
		smc.kubeClient.CoreV1(),
		smc.configMapLister,
		smc.eventRecorder,
	)
	for _, ccc := range certChainConfigs {
		applyErrors = append(applyErrors, cm.ManageCertificateChain(
			ctx,
			time.Now,
			&sm.ObjectMeta,
			scylladbMonitoringControllerGVK,
			ccc,
			secrets,
			configMaps,
		))
	}

	applyError := kutilerrors.NewAggregate(applyErrors)
	if applyError != nil {
		return progressingConditions, applyError
	}

	return progressingConditions, nil
}
