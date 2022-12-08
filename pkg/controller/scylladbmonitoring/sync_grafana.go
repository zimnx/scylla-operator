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
	"k8s.io/utils/pointer"
)

func (smc *Controller) getGrafanaLabels(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Set {
	return labels.Set{
		naming.ScyllaDBMonitoringNameLabel: sm.Name,
		naming.ControllerNameLabel:         "grafana",
	}
}

func (smc *Controller) getGrafanaSelector(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Selector {
	return labels.SelectorFromSet(smc.getGrafanaLabels(sm))
}

func makeGrafana(sm *scyllav1alpha1.ScyllaDBMonitoring) (*integreatlyv1alpha1.Grafana, string, error) {
	return grafanav1alpha1assets.GrafanaTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaOverviewDashboard(sm *scyllav1alpha1.ScyllaDBMonitoring) (*integreatlyv1alpha1.GrafanaDashboard, string, error) {
	return grafanav1alpha1assets.GrafanaOverviewDashboardTemplate.RenderObject(map[string]any{
		"scyllaDBMonitoringName": sm.Name,
	})
}

func makeGrafanaIngress(sm *scyllav1alpha1.ScyllaDBMonitoring) (*networkingv1.Ingress, string, error) {
	var ingressOptions *scyllav1alpha1.IngressOptions
	if sm.Spec.Components != nil && sm.Spec.Components.Grafana != nil && sm.Spec.Components.Grafana.ExposeOptions != nil && sm.Spec.Components.Grafana.ExposeOptions.WebInterface != nil && sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress != nil {
		ingressOptions = sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress
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
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	dashboards, err := controllerhelpers.GetObjects[CT, *integreatlyv1alpha1.GrafanaDashboard](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getGrafanaSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *integreatlyv1alpha1.GrafanaDashboard]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.grafanaDashboardLister.List,
			PatchObjectFunc:           smc.integreatlyClient.GrafanaDashboards(sm.Namespace).Patch,
		},
	)

	datasources, err := controllerhelpers.GetObjects[CT, *integreatlyv1alpha1.GrafanaDataSource](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getGrafanaSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *integreatlyv1alpha1.GrafanaDataSource]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.grafanaDataSourceLister.List,
			PatchObjectFunc:           smc.integreatlyClient.GrafanaDataSources(sm.Namespace).Patch,
		},
	)

	ingresses, err := controllerhelpers.GetObjects[CT, *networkingv1.Ingress](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getGrafanaSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *networkingv1.Ingress]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.ingressLister.List,
			PatchObjectFunc:           smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Patch,
		},
	)

	// FIXME: we can't prune secrets because they are managed separately and we don't know the desired ones
	secrets, err := controllerhelpers.GetObjects[CT, *corev1.Secret](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getGrafanaSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Secret]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.secretLister.List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().Secrets(sm.Namespace).Patch,
		},
	)

	// FIXME: we can't prune configMaps because they are managed separately and we don't know the desired ones
	configMaps, err := controllerhelpers.GetObjects[CT, *corev1.ConfigMap](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getGrafanaSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ConfigMap]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.configMapLister.List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Patch,
		},
	)

	// Render manifests.
	var renderErrors []error

	requiredOverviewDashboard, _, err := makeGrafanaOverviewDashboard(sm)
	renderErrors = append(renderErrors, err)

	requiredIngress, _, err := makeGrafanaIngress(sm)
	renderErrors = append(renderErrors, err)

	requiredPrometheusDatasource, _, err := makeGrafanaPrometheusDataSource(sm)
	renderErrors = append(renderErrors, err)

	requiredGrafana, _, err := makeGrafana(sm)
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
			item.required.SetLabels(smc.getGrafanaLabels(sm))
		} else {
			resourcemerge.MergeMapInPlaceWithoutRemovalKeys2(item.required.GetLabels(), smc.getGrafanaLabels(sm))
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

	if sm.Spec.Components != nil && sm.Spec.Components.Grafana != nil && sm.Spec.Components.Grafana.ExposeOptions != nil && sm.Spec.Components.Grafana.ExposeOptions.WebInterface != nil && sm.Spec.Components.Grafana.ExposeOptions.WebInterface.Ingress != nil {
		applyErrors = append(applyErrors, cm.ManageCertificates(
			ctx,
			time.Now,
			&sm.ObjectMeta,
			scylladbMonitoringControllerGVK,
			&okubecrypto.CAConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   fmt.Sprintf("%s-grafana-serving-ca", sm.Name),
					Labels: smc.getGrafanaLabels(sm),
				},
				Validity: 10 * 365 * 24 * time.Hour,
				Refresh:  8 * 365 * 24 * time.Hour,
			},
			&okubecrypto.CABundleConfig{
				MetaConfig: okubecrypto.MetaConfig{
					Name:   fmt.Sprintf("%s-grafana-serving-ca", sm.Name),
					Labels: smc.getGrafanaLabels(sm),
				},
			},
			[]*okubecrypto.CertificateConfig{
				{
					MetaConfig: okubecrypto.MetaConfig{
						Name:   fmt.Sprintf("%s-grafana-serving-certs", sm.Name),
						Labels: smc.getGrafanaLabels(sm),
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
