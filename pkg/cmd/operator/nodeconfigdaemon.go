// Copyright (C) 2021 ScyllaDB

package operator

import (
	"context"
	"fmt"
	"time"

	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllainformers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controller/nodeconfigdaemon"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	resyncPeriod = 12 * time.Hour
)

type NodeConfigOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	NodeName             string
	NodeConfigUID        string
	ScyllaImage          string
	DisableOptimizations bool

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewNodeConfigOptions(streams genericclioptions.IOStreams) *NodeConfigOptions {
	return &NodeConfigOptions{
		ClientConfig:        genericclioptions.NewClientConfig("node-config"),
		InClusterReflection: genericclioptions.InClusterReflection{},
	}
}

func NewNodeConfigCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeConfigOptions(streams)

	cmd := &cobra.Command{
		Use:   "node-config-daemon",
		Short: "Runs a controller for a particular Kubernetes node.",
		Long:  "Runs a controller for a particular Kubernetes node that configures the node and adjusts configuration that depends on active pods on this node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd.Name())
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
	cmd.Flags().StringVarP(&o.NodeConfigUID, "node-config-uid", "", o.NodeConfigUID, "UID of the NodeConfig that owns this subcontroller.")
	cmd.Flags().StringVarP(&o.ScyllaImage, "scylla-image", "", o.ScyllaImage, "Scylla image used for running perftune.")
	cmd.Flags().BoolVarP(&o.DisableOptimizations, "disable-optimizations", "", o.DisableOptimizations, "Controls if optimizations are disabled")

	return cmd
}

func (o *NodeConfigOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())

	if len(o.NodeName) == 0 {
		errs = append(errs, fmt.Errorf("node-name cannot be empty"))
	}
	if len(o.NodeConfigUID) == 0 {
		errs = append(errs, fmt.Errorf("node-config-uid cannot be empty"))
	}
	if len(o.ScyllaImage) == 0 {
		errs = append(errs, fmt.Errorf("scylla-image cannot be empty"))
	}

	return apierrors.NewAggregate(errs)
}

func (o *NodeConfigOptions) Complete() error {
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

func (o *NodeConfigOptions) Run(streams genericclioptions.IOStreams, commandName string) error {
	klog.Infof("%s version %s", commandName, version.Get())
	klog.Infof("loglevel is set to %q", cmdutil.GetLoglevel())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	criClient, err := cri.NewClient(ctx, naming.HostFilesystemDirName)
	if err != nil {
		return err
	}

	singleNamespaceInformer := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, resyncPeriod, informers.WithNamespace(naming.ScyllaOperatorNodeTuningNamespace))
	nodeConfigInformer := scyllainformers.NewSharedInformerFactory(o.scyllaClient, resyncPeriod)
	singleNodeScyllaPodsInformer := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, resyncPeriod, informers.WithTweakListOptions(
		func(options *metav1.ListOptions) {
			options.LabelSelector = naming.ScyllaSelector().String()
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", o.NodeName).String()
		},
	))

	ntc, err := nodeconfigdaemon.NewController(
		o.kubeClient,
		o.scyllaClient,
		criClient,
		nodeConfigInformer.Scylla().V1alpha1().NodeConfigs(),
		singleNodeScyllaPodsInformer.Core().V1().Pods(),
		singleNamespaceInformer.Apps().V1().DaemonSets(),
		singleNamespaceInformer.Batch().V1().Jobs(),
		o.NodeName,
		types.UID(o.NodeConfigUID),
		o.ScyllaImage,
	)
	if err != nil {
		return fmt.Errorf("can't create node config instance controller: %w", err)
	}

	// Start informers.
	nodeConfigInformer.Start(ctx.Done())
	singleNodeScyllaPodsInformer.Start(ctx.Done())
	singleNamespaceInformer.Start(ctx.Done())

	ntc.Run(ctx)

	<-ctx.Done()

	return nil
}
