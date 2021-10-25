// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfig/resource"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/naming"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
)

func (ncc *Controller) sync(ctx context.Context, key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("can't split meta namespace cache key: %w", err)
	}

	snc, err := ncc.scyllaNodeConfigLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) && name == resource.DefaultScyllaNodeConfig().Name {
			snc, err = ncc.scyllaClient.NodeConfigs().Create(ctx, resource.DefaultScyllaNodeConfig(), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create default NodeConfig: %w", err)
			}
			return nil
		} else {
			return fmt.Errorf("get NodeConfig %q: %w", name, err)
		}
	}

	soc, err := ncc.scyllaOperatorConfigLister.Get(naming.ScyllaOperatorName)
	if err != nil {
		return fmt.Errorf("get ScyllaOperatorConfig: %w", err)
	}

	scyllaPods, err := ncc.podLister.Pods(corev1.NamespaceAll).List(naming.ScyllaSelector())
	if err != nil {
		return fmt.Errorf("get Scylla Pods: %w", err)
	}

	namespaces, err := ncc.getNamespaces()
	if err != nil {
		return fmt.Errorf("get Namespaces: %w", err)
	}

	clusterRoles, err := ncc.getClusterRoles()
	if err != nil {
		return fmt.Errorf("get ClusterRoles: %w", err)
	}

	serviceAccounts, err := ncc.getServiceAccounts()
	if err != nil {
		return fmt.Errorf("get ServiceAccounts: %w", err)
	}

	clusterRoleBindings, err := ncc.getClusterRoleBindings()
	if err != nil {
		return fmt.Errorf("get ClusterRoleBindings: %w", err)
	}

	daemonSets, err := ncc.getDaemonSets(ctx, snc)
	if err != nil {
		return fmt.Errorf("get DaemonSets: %w", err)
	}

	jobs, err := ncc.getJobs(ctx, snc)
	if err != nil {
		return fmt.Errorf("get Jobs: %w", err)
	}

	configMaps, err := ncc.getConfigMaps(ctx, snc, scyllaPods)
	if err != nil {
		return fmt.Errorf("get ConfigMaps: %w", err)
	}

	status := ncc.calculateStatus(snc, daemonSets)

	if snc.DeletionTimestamp != nil {
		return ncc.updateStatus(ctx, snc, status)
	}

	var errs []error

	if err := ncc.syncNamespaces(ctx, namespaces); err != nil {
		errs = append(errs, fmt.Errorf("sync Namespace(s): %w", err))
	}

	if err := ncc.syncClusterRoles(ctx, clusterRoles); err != nil {
		errs = append(errs, fmt.Errorf("sync ClusterRole(s): %w", err))
	}

	if err := ncc.syncServiceAccounts(ctx, serviceAccounts); err != nil {
		errs = append(errs, fmt.Errorf("sync ServiceAccount(s): %w", err))
	}

	if err := ncc.syncClusterRoleBindings(ctx, clusterRoleBindings); err != nil {
		errs = append(errs, fmt.Errorf("sync ClusterRoleBinding(s): %w", err))
	}

	if err := ncc.syncDaemonSets(ctx, snc, soc, status, daemonSets); err != nil {
		errs = append(errs, fmt.Errorf("sync DaemonSet(s): %w", err))
	}

	if err := ncc.syncConfigMaps(ctx, snc, scyllaPods, configMaps, jobs); err != nil {
		errs = append(errs, fmt.Errorf("sync ConfigMap(s): %w", err))
	}

	err = ncc.updateStatus(ctx, snc, status)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}

func (ncc *Controller) getDaemonSets(ctx context.Context, snc *scyllav1alpha1.NodeConfig) (map[string]*appsv1.DaemonSet, error) {
	dss, err := ncc.daemonSetLister.DaemonSets(naming.ScyllaOperatorNodeTuningNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list daemonsets: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: snc.Name,
	})

	canAdoptFunc := func() error {
		fresh, err := ncc.scyllaClient.NodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != snc.UID {
			return fmt.Errorf("original NodeConfig %q is gone: got uid %v, wanted %v", snc.Name, fresh.UID, snc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%q has just been deleted at %v", snc.Name, snc.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewDaemonSetControllerRefManager(
		ctx,
		snc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealDaemonSetControl{
			KubeClient: ncc.kubeClient,
			Recorder:   ncc.eventRecorder,
		},
	)
	return cm.ClaimDaemonSets(dss)
}

func (ncc *Controller) getNamespaces() (map[string]*corev1.Namespace, error) {
	nss, err := ncc.namespaceLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	nsMap := map[string]*corev1.Namespace{}
	for i := range nss {
		nsMap[nss[i].Name] = nss[i]
	}

	return nsMap, nil
}

func (ncc *Controller) getClusterRoles() (map[string]*rbacv1.ClusterRole, error) {
	crs, err := ncc.clusterRoleLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list clusterroles: %w", err)
	}

	crMap := map[string]*rbacv1.ClusterRole{}
	for i := range crs {
		crMap[crs[i].Name] = crs[i]
	}

	return crMap, nil
}

func (ncc *Controller) getClusterRoleBindings() (map[string]*rbacv1.ClusterRoleBinding, error) {
	crbs, err := ncc.clusterRoleBindingLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list clusterrolebindings: %w", err)
	}

	crbMap := map[string]*rbacv1.ClusterRoleBinding{}
	for i := range crbs {
		crbMap[crbs[i].Name] = crbs[i]
	}

	return crbMap, nil
}

func (ncc *Controller) getServiceAccounts() (map[string]*corev1.ServiceAccount, error) {
	sas, err := ncc.serviceAccountLister.List(labels.SelectorFromSet(map[string]string{
		naming.NodeConfigNameLabel: naming.NodeConfigAppName,
	}))
	if err != nil {
		return nil, fmt.Errorf("list serviceaccounts: %w", err)
	}

	sasMap := map[string]*corev1.ServiceAccount{}
	for i := range sas {
		sasMap[sas[i].Name] = sas[i]
	}

	return sasMap, nil
}

func (ncc *Controller) getJobs(ctx context.Context, snc *scyllav1alpha1.NodeConfig) (map[string]*batchv1.Job, error) {
	jobs, err := ncc.jobLister.Jobs(naming.ScyllaOperatorNodeTuningNamespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: snc.Name,
	})

	canAdoptFunc := func() error {
		fresh, err := ncc.scyllaClient.NodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != snc.UID {
			return fmt.Errorf("original NodeConfig %q is gone: got uid %v, wanted %v", snc.Name, fresh.UID, snc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%q has just been deleted at %v", snc.Name, snc.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewJobControllerRefManager(
		ctx,
		snc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealJobControl{
			KubeClient: ncc.kubeClient,
			Recorder:   ncc.eventRecorder,
		},
	)
	return cm.ClaimJobs(jobs)
}

func (ncc *Controller) getConfigMaps(ctx context.Context, snc *scyllav1alpha1.NodeConfig, scyllaPods []*corev1.Pod) (map[string]*corev1.ConfigMap, error) {
	var configMaps []*corev1.ConfigMap

	namespaces := map[string]struct{}{}
	for _, scyllaPod := range scyllaPods {
		namespaces[scyllaPod.Namespace] = struct{}{}
	}

	for ns := range namespaces {
		cms, err := ncc.configMapLister.ConfigMaps(ns).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("list configmaps: %w", err)
		}
		configMaps = append(configMaps, cms...)
	}

	selector := labels.SelectorFromSet(labels.Set{
		naming.NodeConfigNameLabel: snc.Name,
	})

	canAdoptFunc := func() error {
		fresh, err := ncc.scyllaClient.NodeConfigs().Get(ctx, snc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if fresh.UID != snc.UID {
			return fmt.Errorf("original NodeConfig %q is gone: got uid %v, wanted %v", snc.Name, fresh.UID, snc.UID)
		}

		if fresh.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%q has just been deleted at %v", snc.Name, snc.DeletionTimestamp)
		}

		return nil
	}

	cm := controllertools.NewConfigMapControllerRefManager(
		ctx,
		snc,
		controllerGVK,
		selector,
		canAdoptFunc,
		controllertools.RealConfigMapControl{
			KubeClient: ncc.kubeClient,
			Recorder:   ncc.eventRecorder,
		},
	)
	return cm.ClaimConfigMaps(configMaps)
}
