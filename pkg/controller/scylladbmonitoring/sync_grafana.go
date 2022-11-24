package scylladbmonitoring

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"time"

	grafanav1alpha1assets "github.com/scylladb/scylla-operator/assets/monitoring/grafana/v1alpha1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	ocrypto "github.com/scylladb/scylla-operator/pkg/crypto"
	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	okubecrypto "github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/pointer"
)

const (
	grafanaPasswordLength = 20
)

func getGrafanaLabels(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Set {
	return helpers.MergeMaps(
		getLabels(sm),
		labels.Set{
			naming.ControllerNameLabel: "grafana",
		},
	)
}

func getGrafanaSelector(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Selector {
	return labels.SelectorFromSet(getGrafanaLabels(sm))
}

func generateGrafanaPassword() string {
	return rand.String(grafanaPasswordLength)
}

func makeGrafana(sm *scyllav1alpha1.ScyllaDBMonitoring, grafanas map[string]*integreatlyv1alpha1.Grafana, grafanaServingCertSecretName string) (*integreatlyv1alpha1.Grafana, error) {
	var affinity corev1.Affinity
	var tolerations []corev1.Toleration
	if sm.Spec.Components != nil && sm.Spec.Components.Grafana != nil && sm.Spec.Components.Grafana.Placement != nil {
		affinity.NodeAffinity = sm.Spec.Components.Grafana.Placement.NodeAffinity
		affinity.PodAffinity = sm.Spec.Components.Grafana.Placement.PodAffinity
		affinity.PodAntiAffinity = sm.Spec.Components.Grafana.Placement.PodAntiAffinity

		tolerations = sm.Spec.Components.Grafana.Placement.Tolerations
	}

	var resources corev1.ResourceRequirements
	if sm.Spec.Components != nil && sm.Spec.Components.Grafana != nil {
		resources = sm.Spec.Components.Grafana.Resources
	}

	required, _, err := grafanav1alpha1assets.GrafanaTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"password":               "::to::be:replaced::",
		"servingCertSecretName":  grafanaServingCertSecretName,
		"affinity":               affinity,
		"tolerations":            tolerations,
		"resources":              resources,
	})
	if err != nil {
		return required, err
	}

	existing, ok := grafanas[required.Name]
	if ok && existing.Spec.Config.Security != nil && len(existing.Spec.Config.Security.AdminPassword) == grafanaPasswordLength {
		required.Spec.Config.Security.AdminPassword = existing.Spec.Config.Security.AdminPassword

		return required, nil
	}

	required.Spec.Config.Security.AdminPassword = generateGrafanaPassword()

	return required, nil
}

func makeGrafanaAccessSecret(sm *scyllav1alpha1.ScyllaDBMonitoring, grafana *integreatlyv1alpha1.Grafana) (*corev1.Secret, string, error) {
	var username, password []byte
	if grafana != nil && grafana.Spec.Config.Security != nil {
		username = []byte(grafana.Spec.Config.Security.AdminUser)
		password = []byte(grafana.Spec.Config.Security.AdminPassword)
	}

	return grafanav1alpha1assets.GrafanaAccessCredentialsSecretTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"username":               username,
		"password":               password,
	})
}

func makeGrafanaOverviewDashboardConfigMap(sm *scyllav1alpha1.ScyllaDBMonitoring) (*corev1.ConfigMap, string, error) {
	return grafanav1alpha1assets.GrafanaOverviewDashboardConfigMapTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaScyllaDBFolder(sm *scyllav1alpha1.ScyllaDBMonitoring) (*integreatlyv1alpha1.GrafanaFolder, string, error) {
	return grafanav1alpha1assets.GrafanaScyllaDBFolderTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaOverviewDashboard(sm *scyllav1alpha1.ScyllaDBMonitoring) (*integreatlyv1alpha1.GrafanaDashboard, string, error) {
	return grafanav1alpha1assets.GrafanaOverviewDashboardTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaIngress(sm *scyllav1alpha1.ScyllaDBMonitoring) (*networkingv1.Ingress, string, error) {
	var ingressOptions scyllav1alpha1.IngressOptions
	if sm.Spec.Components != nil && sm.Spec.Components.Grafana != nil && sm.Spec.Components.Grafana.ExposeOptions != nil && sm.Spec.Components.Grafana.ExposeOptions.WebInterface != nil && sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress != nil {
		ingressOptions = *sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress
	}
	return grafanav1alpha1assets.GrafanaIngressTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
		"ingressOptions":         ingressOptions,
	})
}

func makeGrafanaPrometheusDataSource(sm *scyllav1alpha1.ScyllaDBMonitoring) (*integreatlyv1alpha1.GrafanaDataSource, string, error) {
	return grafanav1alpha1assets.GrafanaPrometheusDatasourceTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func (smc *Controller) syncGrafana(
	ctx context.Context,
	sm *scyllav1alpha1.ScyllaDBMonitoring,
	grafanas map[string]*integreatlyv1alpha1.Grafana,
	grafanaFolders map[string]*integreatlyv1alpha1.GrafanaFolder,
	dashboards map[string]*integreatlyv1alpha1.GrafanaDashboard,
	datasources map[string]*integreatlyv1alpha1.GrafanaDataSource,
	secrets map[string]*corev1.Secret,
	configMaps map[string]*corev1.ConfigMap,
	ingresses map[string]*networkingv1.Ingress,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	grafanaServingCertChainConfig := &okubecrypto.CertChainConfig{
		CAConfig: &okubecrypto.CAConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-grafana-serving-ca", sm.Name),
				Labels: getGrafanaLabels(sm),
			},
			Validity: 10 * 365 * 24 * time.Hour,
			Refresh:  8 * 365 * 24 * time.Hour,
		},
		CABundleConfig: &okubecrypto.CABundleConfig{
			MetaConfig: okubecrypto.MetaConfig{
				Name:   fmt.Sprintf("%s-grafana-serving-ca", sm.Name),
				Labels: getGrafanaLabels(sm),
			},
		},
		CertConfigs: []*okubecrypto.CertificateConfig{
			{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   fmt.Sprintf("%s-grafana-serving-certs", sm.Name),
					Labels: getGrafanaLabels(sm),
				},
				Validity: 30 * 24 * time.Hour,
				Refresh:  20 * 24 * time.Hour,
				CertCreator: (&ocrypto.ServingCertCreatorConfig{
					Subject: pkix.Name{
						CommonName: "",
					},
					IPAddresses: nil,
					DNSNames:    sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress.DNSDomains,
				}).ToCreator(),
			},
		},
	}

	var certChainConfigs okubecrypto.CertChainConfigs

	var grafanaServingCertSecretName string
	if sm.Spec.Components != nil && sm.Spec.Components.Grafana != nil {
		grafanaServingCertSecretName = sm.Spec.Components.Grafana.ServingCertSecretName
	}

	if len(grafanaServingCertSecretName) == 0 {
		grafanaServingCertSecretName = grafanaServingCertChainConfig.CertConfigs[0].Name
		certChainConfigs = append(certChainConfigs, grafanaServingCertChainConfig)
	}

	// Render manifests.
	var renderErrors []error

	requiredScyllaDBGrafanaFolder, _, err := makeGrafanaScyllaDBFolder(sm)
	renderErrors = append(renderErrors, err)

	requiredOverviewDashboardConfigMap, _, err := makeGrafanaOverviewDashboardConfigMap(sm)
	renderErrors = append(renderErrors, err)

	requiredOverviewDashboard, _, err := makeGrafanaOverviewDashboard(sm)
	renderErrors = append(renderErrors, err)

	requiredIngress, _, err := makeGrafanaIngress(sm)
	renderErrors = append(renderErrors, err)

	requiredPrometheusDatasource, _, err := makeGrafanaPrometheusDataSource(sm)
	renderErrors = append(renderErrors, err)

	requiredGrafana, err := makeGrafana(sm, grafanas, grafanaServingCertSecretName)
	renderErrors = append(renderErrors, err)

	requiredGrafanaAccessSecret, _, err := makeGrafanaAccessSecret(sm, requiredGrafana)
	renderErrors = append(renderErrors, err)

	renderError := kutilerrors.NewAggregate(renderErrors)
	if renderError != nil {
		return progressingConditions, renderError
	}

	// Prune objects.
	var pruneErrors []error

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredOverviewDashboard),
		dashboards,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.integreatlyClient.GrafanaDashboards(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredIngress),
		ingresses,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredScyllaDBGrafanaFolder),
		grafanaFolders,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.integreatlyClient.GrafanaFolders(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredPrometheusDatasource),
		datasources,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.integreatlyClient.GrafanaDataSources(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		helpers.ToArray(requiredGrafana),
		grafanas,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.integreatlyClient.Grafanas(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		append([]*corev1.Secret{requiredGrafanaAccessSecret}, certChainConfigs.GetMetaSecrets()...),
		secrets,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.kubeClient.CoreV1().Secrets(sm.Namespace).Delete,
		},
	)
	pruneErrors = append(pruneErrors, err)

	err = controllerhelpers.Prune(
		ctx,
		append(
			[]*corev1.ConfigMap{
				requiredOverviewDashboardConfigMap,
			},
			certChainConfigs.GetMetaConfigMaps()...,
		),
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
			required: requiredOverviewDashboardConfigMap,
			control: resourceapply.ApplyControlFuncs[*corev1.ConfigMap]{
				GetCachedFunc: smc.configMapLister.ConfigMaps(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredGrafanaAccessSecret,
			control: resourceapply.ApplyControlFuncs[*corev1.Secret]{
				GetCachedFunc: smc.secretLister.Secrets(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().Secrets(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().Secrets(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredScyllaDBGrafanaFolder,
			control: resourceapply.ApplyControlFuncs[*integreatlyv1alpha1.GrafanaFolder]{
				GetCachedFunc: smc.grafanaFolderLister.GrafanaFolders(sm.Namespace).Get,
				CreateFunc:    smc.integreatlyClient.GrafanaFolders(sm.Namespace).Create,
				UpdateFunc:    smc.integreatlyClient.GrafanaFolders(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredOverviewDashboard,
			control: resourceapply.ApplyControlFuncs[*integreatlyv1alpha1.GrafanaDashboard]{
				GetCachedFunc: smc.grafanaDashboardLister.GrafanaDashboards(sm.Namespace).Get,
				CreateFunc:    smc.integreatlyClient.GrafanaDashboards(sm.Namespace).Create,
				UpdateFunc:    smc.integreatlyClient.GrafanaDashboards(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredPrometheusDatasource,
			control: resourceapply.ApplyControlFuncs[*integreatlyv1alpha1.GrafanaDataSource]{
				GetCachedFunc: smc.grafanaDataSourceLister.GrafanaDataSources(sm.Namespace).Get,
				CreateFunc:    smc.integreatlyClient.GrafanaDataSources(sm.Namespace).Create,
				UpdateFunc:    smc.integreatlyClient.GrafanaDataSources(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredIngress,
			control: resourceapply.ApplyControlFuncs[*networkingv1.Ingress]{
				GetCachedFunc: smc.ingressLister.Ingresses(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredGrafana,
			control: resourceapply.ApplyControlFuncs[*integreatlyv1alpha1.Grafana]{
				GetCachedFunc: smc.grafanaLister.Grafanas(sm.Namespace).Get,
				CreateFunc:    smc.integreatlyClient.Grafanas(sm.Namespace).Create,
				UpdateFunc:    smc.integreatlyClient.Grafanas(sm.Namespace).Update,
			}.ToUntyped(),
		},
	} {
		// Enforce namespace.
		item.required.SetNamespace(sm.Namespace)

		// Enforce labels for selection.
		if item.required.GetLabels() == nil {
			item.required.SetLabels(getGrafanaLabels(sm))
		} else {
			resourcemerge.MergeMapInPlaceWithoutRemovalKeys2(item.required.GetLabels(), getGrafanaLabels(sm))
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
			controllerhelpers.AddGenericProgressingStatusCondition(&progressingConditions, grafanaControllerProgressingCondition, item.required, "apply", sm.Generation)
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
