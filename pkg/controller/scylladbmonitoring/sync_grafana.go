package scylladbmonitoring

import (
	"context"
	"fmt"

	grafanav1alpha1assets "github.com/scylladb/scylla-operator/assets/monitoring/grafana/v1alpha1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
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
		"namespace":              sm.Namespace,
		"scyllaDBMonitoringName": sm.Name,
	})
}

func (smc *Controller) syncGrafana(
	ctx context.Context,
	sm *scyllav1alpha1.ScyllaDBMonitoring,
	grafanas map[string]*integreatlyv1alpha1.Grafana,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	serviceAccounts, err := controllerhelpers.GetObjects[CT, *corev1.ServiceAccount](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getGrafanaSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ServiceAccount]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.serviceAccountLister.List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Patch,
		},
	)

	roleBindings, err := controllerhelpers.GetObjects[CT, *rbacv1.RoleBinding](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getGrafanaSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *rbacv1.RoleBinding]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.roleBindingLister.List,
			PatchObjectFunc:           smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Patch,
		},
	)

	// Render manifests.
	var renderErrors []error

	requiredGrafana, _, err := makeGrafana(sm)
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
		helpers.ToArray(requiredGrafana),
		grafanas,
		&controllerhelpers.PruneControlFuncs{
			DeleteFunc: smc.monitoringClient.Grafanas(sm.Namespace).Delete,
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
			required: requiredGrafanaSA,
			control: resourceapply.ApplyControlFuncs[*corev1.ServiceAccount]{
				GetCachedFunc: smc.serviceAccountLister.ServiceAccounts(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredGrafanaService,
			control: resourceapply.ApplyControlFuncs[*corev1.Service]{
				GetCachedFunc: smc.serviceLister.Services(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.CoreV1().Services(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredGrafanaRoleBinding,
			control: resourceapply.ApplyControlFuncs[*rbacv1.RoleBinding]{
				GetCachedFunc: smc.roleBindingLister.RoleBindings(sm.Namespace).Get,
				CreateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Create,
				UpdateFunc:    smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Update,
			}.ToUntyped(),
		},
		{
			required: requiredGrafana,
			control: resourceapply.ApplyControlFuncs[*monitoringv1.Grafana]{
				GetCachedFunc: smc.grafanaLister.Grafanaes(sm.Namespace).Get,
				CreateFunc:    smc.monitoringClient.Grafanaes(sm.Namespace).Create,
				UpdateFunc:    smc.monitoringClient.Grafanaes(sm.Namespace).Update,
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

	applyError := kutilerrors.NewAggregate(applyErrors)
	if applyError != nil {
		return progressingConditions, applyError
	}

	return progressingConditions, nil
}
