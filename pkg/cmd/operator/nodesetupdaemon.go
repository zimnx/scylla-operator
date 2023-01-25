// Copyright (c) 2023 ScyllaDB.

package operator

import (
	"context"
	"fmt"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/controller/nodesetup"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type NodeSetupDaemonOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	NodeName       string
	NodeConfigName string
	NodeConfigUID  string

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewNodeSetupOptions(streams genericclioptions.IOStreams) *NodeSetupDaemonOptions {
	return &NodeSetupDaemonOptions{
		ClientConfig:        genericclioptions.NewClientConfig("node-setup"),
		InClusterReflection: genericclioptions.InClusterReflection{},
	}
}

func NewNodeSetupCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeSetupOptions(streams)

	cmd := &cobra.Command{
		Use:   "node-setup-daemon",
		Short: "Runs a controller for a particular Kubernetes node.",
		Long:  "Runs a controller for a particular Kubernetes node that configures the node and adjusts configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	o.ClientConfig.AddFlags(cmd)
	o.InClusterReflection.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.NodeName, "node-name", "", o.NodeName, "Name of the node where this Pod is running.")
	cmd.Flags().StringVarP(&o.NodeConfigName, "node-config-name", "", o.NodeConfigName, "Name of the NodeConfig that owns this subcontroller.")
	cmd.Flags().StringVarP(&o.NodeConfigUID, "node-config-uid", "", o.NodeConfigUID, "UID of the NodeConfig that owns this subcontroller.")

	return cmd
}

func (o *NodeSetupDaemonOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())

	if len(o.NodeName) == 0 {
		errs = append(errs, fmt.Errorf("node-name can't be empty"))
	}

	if len(o.NodeConfigName) == 0 {
		errs = append(errs, fmt.Errorf("node-config-name can't be empty"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *NodeSetupDaemonOptions) Complete() error {
	err := o.ClientConfig.Complete()
	if err != nil {
		return err
	}

	err = o.InClusterReflection.Complete()
	if err != nil {
		return err
	}

	o.kubeClient, err = kubernetes.NewForConfig(o.ProtoConfig)
	if err != nil {
		return fmt.Errorf("can't build kubernetes clientset: %w", err)
	}

	o.scyllaClient, err = scyllaversionedclient.NewForConfig(o.RestConfig)
	if err != nil {
		return fmt.Errorf("can't build scylla clientset: %w", err)
	}

	return nil
}

func (o *NodeSetupDaemonOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	nodeConfigInformers := scyllainformers.NewSharedInformerFactory(o.scyllaClient, resyncPeriod)

	var err error
	var node *corev1.Node
	err = wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
		node, err = o.kubeClient.CoreV1().Nodes().Get(ctx, o.NodeName, metav1.GetOptions{})
		if err != nil {
			klog.V(2).InfoS("Can't get Node", "Node", o.NodeName, "Error", err.Error())
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("can't get node %q: %w", o.NodeName, err)
	}

	ncdc, err := nodesetup.NewController(
		o.kubeClient,
		o.scyllaClient.ScyllaV1alpha1(),
		nodeConfigInformers.Scylla().V1alpha1().NodeConfigs(),
		node.Name,
		node.UID,
		o.NodeConfigName,
		types.UID(o.NodeConfigUID),
	)
	if err != nil {
		return fmt.Errorf("can't create node config instance controller: %w", err)
	}

	// Start informers.
	nodeConfigInformers.Start(ctx.Done())

	// Run the controller to configure and reconcile pod specific options.
	ncdc.Run(ctx)

	<-ctx.Done()

	return nil
}
