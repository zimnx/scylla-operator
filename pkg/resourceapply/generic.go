package resourceapply

import (
	"context"
	"fmt"

	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/resourcemerge"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type ApplyControlUntypedInterface interface {
	GetCached(name string) (kubeinterfaces.ObjectInterface, error)
	Create(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error)
	Update(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error)
}

type ApplyControlUntypedFuncs struct {
	GetCachedFunc func(name string) (kubeinterfaces.ObjectInterface, error)
	CreateFunc    func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error)
	UpdateFunc    func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error)
}

func (acf ApplyControlUntypedFuncs) GetCached(name string) (kubeinterfaces.ObjectInterface, error) {
	return acf.GetCachedFunc(name)
}

func (acf ApplyControlUntypedFuncs) Create(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error) {
	return acf.CreateFunc(ctx, obj, opts)
}

func (acf ApplyControlUntypedFuncs) Update(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error) {
	return acf.UpdateFunc(ctx, obj, opts)
}

var _ ApplyControlUntypedInterface = ApplyControlUntypedFuncs{}

type ApplyControlInterface[T kubeinterfaces.ObjectInterface] interface {
	GetCached(name string) (T, error)
	Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error)
	Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error)
}

type ApplyControlFuncs[T kubeinterfaces.ObjectInterface] struct {
	GetCachedFunc func(name string) (T, error)
	CreateFunc    func(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error)
	UpdateFunc    func(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error)
}

func (acf ApplyControlFuncs[T]) GetCached(name string) (T, error) {
	return acf.GetCachedFunc(name)
}

func (acf ApplyControlFuncs[T]) Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
	return acf.CreateFunc(ctx, obj, opts)
}

func (acf ApplyControlFuncs[T]) Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	return acf.UpdateFunc(ctx, obj, opts)
}

func (acf ApplyControlFuncs[T]) ToUntyped() ApplyControlUntypedFuncs {
	return ApplyControlUntypedFuncs{
		GetCachedFunc: func(name string) (kubeinterfaces.ObjectInterface, error) {
			return acf.GetCached(name)
		},
		CreateFunc: func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.CreateOptions) (kubeinterfaces.ObjectInterface, error) {
			return acf.Create(ctx, obj.(T), opts)
		},
		UpdateFunc: func(ctx context.Context, obj kubeinterfaces.ObjectInterface, opts metav1.UpdateOptions) (kubeinterfaces.ObjectInterface, error) {
			return acf.Update(ctx, obj.(T), opts)
		},
	}
}

var _ ApplyControlInterface[*corev1.Service] = ApplyControlFuncs[*corev1.Service]{}

type ApplyOptions struct {
	ForceOwnership bool
}

func TypeApplyControlInterface[T kubeinterfaces.ObjectInterface](untyped ApplyControlUntypedInterface) ApplyControlInterface[T] {
	return ApplyControlFuncs[T]{
		GetCachedFunc: func(name string) (T, error) {
			res, err := untyped.GetCached(name)
			if res == nil {
				return *new(T), err
			}
			return res.(T), err
		},
		CreateFunc: func(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
			res, err := untyped.Create(ctx, obj, opts)
			if res == nil {
				return *new(T), err
			}
			return res.(T), err
		},
		UpdateFunc: func(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
			res, err := untyped.Update(ctx, obj, opts)
			if res == nil {
				return *new(T), err
			}
			return res.(T), err
		},
	}
}

// func ApplyUntyped(
// 	ctx context.Context,
// 	required kubeinterfaces.ObjectInterface,
// 	control ApplyControlUntypedInterface,
// 	recorder record.EventRecorder,
// 	options ApplyOptions,
// ) (kubeinterfaces.ObjectInterface, bool, error) {
// 	return Apply[required.(type)](ctx, required, control, recorder, options)
//
// 	switch metav1.Object(required).(type) {
// 	case *monitoringv1.Prometheus:
// 		return ApplyGeneric[T](ctx, required, control, recorder, options)
//
// 	default:
// 		return *new(T), false, fmt.Errorf("no apply method matched for type %T", required)
// 	}
// }

func Apply(
	ctx context.Context,
	required kubeinterfaces.ObjectInterface,
	control ApplyControlUntypedInterface,
	recorder record.EventRecorder,
	options ApplyOptions,
) (kubeinterfaces.ObjectInterface, bool, error) {
	switch metav1.Object(required).(type) {
	case *corev1.Service:
		return ApplyService2(
			ctx,
			required.(*corev1.Service),
			TypeApplyControlInterface[*corev1.Service](control),
			recorder,
			options,
		)

	case *corev1.ConfigMap:
		return ApplyConfigMap2(
			ctx,
			required.(*corev1.ConfigMap),
			TypeApplyControlInterface[*corev1.ConfigMap](control),
			recorder,
			options,
		)

	case *corev1.Secret:
		return ApplySecret2(
			ctx,
			required.(*corev1.Secret),
			TypeApplyControlInterface[*corev1.Secret](control),
			recorder,
			options,
		)

	case *corev1.ServiceAccount:
		return ApplyServiceAccount2(
			ctx,
			required.(*corev1.ServiceAccount),
			TypeApplyControlInterface[*corev1.ServiceAccount](control),
			recorder,
			options,
		)

	case *rbacv1.RoleBinding:
		return ApplyRoleBinding2(
			ctx,
			required.(*rbacv1.RoleBinding),
			TypeApplyControlInterface[*rbacv1.RoleBinding](control),
			recorder,
			options,
		)

	case *networkingv1.Ingress:
		return ApplyIngress2(
			ctx,
			required.(*networkingv1.Ingress),
			TypeApplyControlInterface[*networkingv1.Ingress](control),
			recorder,
			options,
		)

	case *monitoringv1.Prometheus:
		return ApplyPrometheus(
			ctx,
			required.(*monitoringv1.Prometheus),
			TypeApplyControlInterface[*monitoringv1.Prometheus](control),
			recorder,
			options,
		)

	case *monitoringv1.PrometheusRule:
		return ApplyPrometheusRule(
			ctx,
			required.(*monitoringv1.PrometheusRule),
			TypeApplyControlInterface[*monitoringv1.PrometheusRule](control),
			recorder,
			options,
		)

	case *monitoringv1.ServiceMonitor:
		return ApplyServiceMonitor(
			ctx,
			required.(*monitoringv1.ServiceMonitor),
			TypeApplyControlInterface[*monitoringv1.ServiceMonitor](control),
			recorder,
			options,
		)

	case *integreatlyv1alpha1.Grafana:
		return ApplyGrafana(
			ctx,
			required.(*integreatlyv1alpha1.Grafana),
			TypeApplyControlInterface[*integreatlyv1alpha1.Grafana](control),
			recorder,
			options,
		)

	case *integreatlyv1alpha1.GrafanaFolder:
		return ApplyGrafanaFolder(
			ctx,
			required.(*integreatlyv1alpha1.GrafanaFolder),
			TypeApplyControlInterface[*integreatlyv1alpha1.GrafanaFolder](control),
			recorder,
			options,
		)

	case *integreatlyv1alpha1.GrafanaDashboard:
		return ApplyGrafanaDashboard(
			ctx,
			required.(*integreatlyv1alpha1.GrafanaDashboard),
			TypeApplyControlInterface[*integreatlyv1alpha1.GrafanaDashboard](control),
			recorder,
			options,
		)

	case *integreatlyv1alpha1.GrafanaDataSource:
		return ApplyGrafanaDataSource(
			ctx,
			required.(*integreatlyv1alpha1.GrafanaDataSource),
			TypeApplyControlInterface[*integreatlyv1alpha1.GrafanaDataSource](control),
			recorder,
			options,
		)

	default:
		return nil, false, fmt.Errorf("no apply method matched for type %T", required)
	}
}

func ApplyPrometheus(
	ctx context.Context,
	required *monitoringv1.Prometheus,
	control ApplyControlInterface[*monitoringv1.Prometheus],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*monitoringv1.Prometheus, bool, error) {
	return ApplyGeneric[*monitoringv1.Prometheus](ctx, required, control, recorder, options)
}

func ApplyPrometheusRule(
	ctx context.Context,
	required *monitoringv1.PrometheusRule,
	control ApplyControlInterface[*monitoringv1.PrometheusRule],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*monitoringv1.PrometheusRule, bool, error) {
	return ApplyGeneric[*monitoringv1.PrometheusRule](ctx, required, control, recorder, options)
}

func ApplyServiceMonitor(
	ctx context.Context,
	required *monitoringv1.ServiceMonitor,
	control ApplyControlInterface[*monitoringv1.ServiceMonitor],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*monitoringv1.ServiceMonitor, bool, error) {
	return ApplyGeneric[*monitoringv1.ServiceMonitor](ctx, required, control, recorder, options)
}

func ApplyGrafana(
	ctx context.Context,
	required *integreatlyv1alpha1.Grafana,
	control ApplyControlInterface[*integreatlyv1alpha1.Grafana],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*integreatlyv1alpha1.Grafana, bool, error) {
	return ApplyGeneric[*integreatlyv1alpha1.Grafana](ctx, required, control, recorder, options)
}

func ApplyGrafanaFolder(
	ctx context.Context,
	required *integreatlyv1alpha1.GrafanaFolder,
	control ApplyControlInterface[*integreatlyv1alpha1.GrafanaFolder],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*integreatlyv1alpha1.GrafanaFolder, bool, error) {
	return ApplyGeneric[*integreatlyv1alpha1.GrafanaFolder](ctx, required, control, recorder, options)
}

func ApplyGrafanaDashboard(
	ctx context.Context,
	required *integreatlyv1alpha1.GrafanaDashboard,
	control ApplyControlInterface[*integreatlyv1alpha1.GrafanaDashboard],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*integreatlyv1alpha1.GrafanaDashboard, bool, error) {
	return ApplyGeneric[*integreatlyv1alpha1.GrafanaDashboard](ctx, required, control, recorder, options)
}

func ApplyGrafanaDataSource(
	ctx context.Context,
	required *integreatlyv1alpha1.GrafanaDataSource,
	control ApplyControlInterface[*integreatlyv1alpha1.GrafanaDataSource],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*integreatlyv1alpha1.GrafanaDataSource, bool, error) {
	return ApplyGeneric[*integreatlyv1alpha1.GrafanaDataSource](ctx, required, control, recorder, options)
}

func ApplyConfigMap2(
	ctx context.Context,
	required *corev1.ConfigMap,
	control ApplyControlInterface[*corev1.ConfigMap],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*corev1.ConfigMap, bool, error) {
	return ApplyGeneric[*corev1.ConfigMap](ctx, required, control, recorder, options)
}

func ApplySecret2(
	ctx context.Context,
	required *corev1.Secret,
	control ApplyControlInterface[*corev1.Secret],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*corev1.Secret, bool, error) {
	return ApplyGeneric[*corev1.Secret](ctx, required, control, recorder, options)
}

func ApplyService2(
	ctx context.Context,
	required *corev1.Service,
	control ApplyControlInterface[*corev1.Service],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*corev1.Service, bool, error) {
	return ApplyGenericWithProjection[*corev1.Service](
		ctx,
		required,
		control,
		recorder,
		options,
		func(required **corev1.Service, existing *corev1.Service) {
			(*required).Spec.ClusterIP = existing.Spec.ClusterIP
			(*required).Spec.ClusterIPs = existing.Spec.ClusterIPs
		},
	)
}

func ApplyServiceAccount2(
	ctx context.Context,
	required *corev1.ServiceAccount,
	control ApplyControlInterface[*corev1.ServiceAccount],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*corev1.ServiceAccount, bool, error) {
	return ApplyGeneric[*corev1.ServiceAccount](ctx, required, control, recorder, options)
}

func ApplyRoleBinding2(
	ctx context.Context,
	required *rbacv1.RoleBinding,
	control ApplyControlInterface[*rbacv1.RoleBinding],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*rbacv1.RoleBinding, bool, error) {
	return ApplyGeneric[*rbacv1.RoleBinding](ctx, required, control, recorder, options)
}

func ApplyIngress2(
	ctx context.Context,
	required *networkingv1.Ingress,
	control ApplyControlInterface[*networkingv1.Ingress],
	recorder record.EventRecorder,
	options ApplyOptions,
) (*networkingv1.Ingress, bool, error) {
	return ApplyGeneric[*networkingv1.Ingress](ctx, required, control, recorder, options)
}

func ApplyGeneric[T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	required T,
	control ApplyControlInterface[T],
	recorder record.EventRecorder,
	options ApplyOptions,
) (T, bool, error) {
	return ApplyGenericWithProjection[T](ctx, required, control, recorder, options, nil)
}

func ApplyGenericWithProjection[T kubeinterfaces.ObjectInterface](
	ctx context.Context,
	required T,
	control ApplyControlInterface[T],
	recorder record.EventRecorder,
	options ApplyOptions,
	projectionFunc func(required *T, existing T),
) (T, bool, error) {
	gvk := resource.GetObjectGVKOrUnknown(required)

	requiredControllerRef := metav1.GetControllerOfNoCopy(required)
	if requiredControllerRef == nil {
		return *new(T), false, fmt.Errorf("%s %q is missing controllerRef", gvk, naming.ObjRef(required))
	}

	requiredCopy := required.DeepCopyObject().(T)
	err := SetHashAnnotation(requiredCopy)
	if err != nil {
		return *new(T), false, err
	}

	existing, err := control.GetCached(requiredCopy.GetName())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return *new(T), false, err
		}

		resourcemerge.SanitizeObject(requiredCopy)
		actual, err := control.Create(
			ctx,
			requiredCopy,
			metav1.CreateOptions{
				FieldValidation: metav1.FieldValidationStrict,
			},
		)
		if apierrors.IsAlreadyExists(err) {
			klog.V(2).InfoS("Already exists (stale cache)", "Service", klog.KObj(requiredCopy))
		} else {
			ReportCreateEvent(recorder, requiredCopy, err)
		}
		return actual, err == nil, err
	}

	existingControllerRef := metav1.GetControllerOfNoCopy(existing)
	if existingControllerRef == nil || existingControllerRef.UID != requiredControllerRef.UID {
		if existingControllerRef == nil && options.ForceOwnership {
			klog.V(2).InfoS("Forcing apply to claim the Service", "Service", naming.ObjRef(requiredCopy))
		} else {
			// This is not the place to handle adoption.
			err := fmt.Errorf("%s %q isn't controlled by us", gvk, naming.ObjRef(requiredCopy))
			ReportUpdateEvent(recorder, requiredCopy, err)
			return *new(T), false, err
		}
	}

	// If they are the same do nothing.
	if existing.GetAnnotations()[naming.ManagedHash] == requiredCopy.GetAnnotations()[naming.ManagedHash] {
		return existing, false, nil
	}

	// TODO: Let this take metav1.Object interface instead.
	resourcemerge.MergeMetadataInPlaceInt(requiredCopy, existing)

	// Project allocated fields, like spec.clusterIP for services.
	if projectionFunc != nil {
		projectionFunc(&requiredCopy, existing)
	}

	// Honor the required RV if it was already set.
	// Required objects set RV in case their input is based on a previous version of itself.
	if len(requiredCopy.GetResourceVersion()) == 0 {
		requiredCopy.SetResourceVersion(existing.GetResourceVersion())
	}

	actual, err := control.Update(
		ctx,
		requiredCopy,
		metav1.UpdateOptions{
			FieldValidation: metav1.FieldValidationStrict,
		},
	)
	if apierrors.IsConflict(err) {
		klog.V(2).InfoS("Hit update conflict, will retry.", "Service", klog.KObj(requiredCopy))
	} else {
		ReportUpdateEvent(recorder, requiredCopy, err)
	}
	if err != nil {
		return *new(T), false, fmt.Errorf("can't update service: %w", err)
	}

	return actual, true, nil
}
