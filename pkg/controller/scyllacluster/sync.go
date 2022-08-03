package scyllacluster

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (sdc *Controller) getStatefulSets(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*appsv1.StatefulSet, error) {
	// List all StatefulSets to find even those that no longer match our selector.
	// They will be orphaned in ClaimStatefulSets().
	statefulSets, err := sdc.statefulSetLister.StatefulSets(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing StatefulSets.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewStatefulSetControllerRefManager(
		ctx,
		sd,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealStatefulSetControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimStatefulSets(statefulSets)
}

func (sdc *Controller) getServices(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*corev1.Service, error) {
	// List all Services to find even those that no longer match our selector.
	// They will be orphaned in ClaimServices().
	services, err := sdc.serviceLister.Services(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Services.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewServiceControllerRefManager(
		ctx,
		sd,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealServiceControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimServices(services)
}

func (sdc *Controller) getSecrets(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*corev1.Secret, error) {
	// List all Secrets to find even those that no longer match our selector.
	// They will be orphaned in ClaimSecrets().
	secrets, err := sdc.secretLister.Secrets(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Secrets.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewSecretControllerRefManager(
		ctx,
		sd,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealSecretControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimSecrets(secrets)
}

func (sdc *Controller) getServiceAccounts(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*corev1.ServiceAccount, error) {
	// List all ServiceAccounts to find even those that no longer match our selector.
	// They will be orphaned in ClaimServiceAccount().
	serviceAccounts, err := sdc.serviceAccountLister.ServiceAccounts(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing StatefulSets.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewServiceAccountControllerRefManager(
		ctx,
		sd,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealServiceAccountControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimServiceAccounts(serviceAccounts)
}

func (sdc *Controller) getRoleBindings(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*rbacv1.RoleBinding, error) {
	// List all RoleBindings to find even those that no longer match our selector.
	// They will be orphaned in ClaimRoleBindings().
	roleBindings, err := sdc.roleBindingLister.RoleBindings(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing RoleBindings.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewRoleBindingControllerRefManager(
		ctx,
		sd,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealRoleBindingControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimRoleBindings(roleBindings)
}

func (sdc *Controller) getPDBs(ctx context.Context, sd *scyllav1alpha1.ScyllaDatacenter) (map[string]*policyv1beta1.PodDisruptionBudget, error) {
	// List all Pdbs to find even those that no longer match our selector.
	// They will be orphaned in ClaimPdbs().
	pdbs, err := sdc.pdbLister.PodDisruptionBudgets(sd.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.ClusterNameLabel: sd.Name,
	})

	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Pdbs.
	canAdoptFunc := func() error {
		fresh, err := sdc.scyllaClient.ScyllaDatacenters(sd.Namespace).Get(ctx, sd.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != sd.UID {
			return fmt.Errorf("original ScyllaDatacenter %v/%v is gone: got uid %v, wanted %v", sd.Namespace, sd.Name, fresh.UID, sd.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", sd.Namespace, sd.Name, sd.DeletionTimestamp)
		}

		return nil
	}
	cm := controllertools.NewPodDisruptionBudgetControllerRefManager(
		ctx,
		sd,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealPodDisruptionBudgetControl{
			KubeClient: sdc.kubeClient,
			Recorder:   sdc.eventRecorder,
		},
	)
	return cm.ClaimPodDisruptionBudgets(pdbs)
}

func (sdc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDatacenter", "ScyllaDatacenter", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDatacenter", "ScyllaDatacenter", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sd, err := sdc.scyllaLister.ScyllaDatacenters(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaDatacenter has been deleted", "ScyllaDatacenter", klog.KObj(sd))
		return nil
	}
	if err != nil {
		return err
	}

	statefulSetMap, err := sdc.getStatefulSets(ctx, sd)
	if err != nil {
		return err
	}

	serviceMap, err := sdc.getServices(ctx, sd)
	if err != nil {
		return err
	}

	secretMap, err := sdc.getSecrets(ctx, sd)
	if err != nil {
		return err
	}

	serviceAccounts, err := sdc.getServiceAccounts(ctx, sd)
	if err != nil {
		return fmt.Errorf("can't get serviceaccounts: %w", err)
	}

	roleBindings, err := sdc.getRoleBindings(ctx, sd)
	if err != nil {
		return fmt.Errorf("can't get rolebindings: %w", err)
	}

	pdbMap, err := sdc.getPDBs(ctx, sd)
	if err != nil {
		return err
	}

	status := sdc.calculateStatus(sd, statefulSetMap, serviceMap)

	if sd.DeletionTimestamp != nil {
		return sdc.updateStatus(ctx, sd, status)
	}

	var errs []error

	err = sdc.syncServiceAccounts(ctx, sd, serviceAccounts)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync serviceaccounts: %w", err))
		// TODO: Set degraded condition
	}

	err = sdc.syncRoleBindings(ctx, sd, roleBindings)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync rolebindings: %w", err))
		// TODO: Set degraded condition
	}

	status, err = sdc.syncAgentToken(ctx, sd, status, secretMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	status, err = sdc.syncStatefulSets(ctx, key, sd, status, statefulSetMap, serviceMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	status, err = sdc.syncServices(ctx, sd, status, serviceMap, statefulSetMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	status, err = sdc.syncPodDisruptionBudgets(ctx, sd, status, pdbMap)
	if err != nil {
		errs = append(errs, err)
		// TODO: Set degraded condition
	}

	err = sdc.updateStatus(ctx, sd, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}
