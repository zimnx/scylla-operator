package operator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"syscall"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaversionedclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	"github.com/scylladb/scylla-operator/pkg/cmdutil"
	"github.com/scylladb/scylla-operator/pkg/controller/helpers"
	sidecarcontroller "github.com/scylladb/scylla-operator/pkg/controller/sidecar"
	"github.com/scylladb/scylla-operator/pkg/genericclioptions"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/sidecar"
	"github.com/scylladb/scylla-operator/pkg/sidecar/config"
	"github.com/scylladb/scylla-operator/pkg/sidecar/identity"
	"github.com/scylladb/scylla-operator/pkg/signals"
	"github.com/scylladb/scylla-operator/pkg/util/network"
	"github.com/scylladb/scylla-operator/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

type SidecarOptions struct {
	genericclioptions.ClientConfig
	genericclioptions.InClusterReflection

	ServiceName string
	SecretName  string
	CPUCount    int

	kubeClient   kubernetes.Interface
	scyllaClient scyllaversionedclient.Interface
}

func NewSidecarOptions(streams genericclioptions.IOStreams) *SidecarOptions {
	clientConfig := genericclioptions.NewClientConfig("scylla-sidecar")
	clientConfig.QPS = 2
	clientConfig.Burst = 5

	return &SidecarOptions{
		ClientConfig:        clientConfig,
		InClusterReflection: genericclioptions.InClusterReflection{},
	}
}

func NewSidecarCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSidecarOptions(streams)

	cmd := &cobra.Command{
		Use:   "sidecar",
		Short: "Run the scylla sidecar.",
		Long:  `Run the scylla sidecar.`,
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

	cmd.Flags().StringVarP(&o.ServiceName, "service-name", "", o.ServiceName, "Name of the service corresponding to the managed node.")
	cmd.Flags().StringVarP(&o.SecretName, "secret-name", "", o.SecretName, "Name of the manager-agent secret for this ScyllaCluster.")
	cmd.Flags().IntVarP(&o.CPUCount, "cpu-count", "", o.CPUCount, "Number of cpus to use.")

	return cmd
}

func (o *SidecarOptions) Validate() error {
	var errs []error

	errs = append(errs, o.ClientConfig.Validate())
	errs = append(errs, o.InClusterReflection.Validate())

	if len(o.ServiceName) == 0 {
		errs = append(errs, fmt.Errorf("service-name can't be empty"))
	} else {
		serviceNameValidationErrs := apimachineryvalidation.NameIsDNS1035Label(o.ServiceName, false)
		if len(serviceNameValidationErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid service name %q: %v", o.ServiceName, serviceNameValidationErrs))
		}
	}

	if len(o.SecretName) == 0 {
		errs = append(errs, fmt.Errorf("secret-name can't be empty"))
	} else {
		secretNameValidationErrs := apimachineryvalidation.NameIsDNSSubdomain(o.SecretName, false)
		if len(secretNameValidationErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid secret name %q: %v", o.SecretName, secretNameValidationErrs))
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (o *SidecarOptions) Complete() error {
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

func (o *SidecarOptions) Run(streams genericclioptions.IOStreams, commandName string) error {
	klog.Infof("%s version %s", commandName, version.Get())
	klog.Infof("loglevel is set to %q", cmdutil.GetLoglevel())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	singleServiceKubeInformers := informers.NewSharedInformerFactoryWithOptions(
		o.kubeClient,
		12*time.Hour,
		informers.WithNamespace(o.Namespace),
		informers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = fields.OneTermEqualSelector("metadata.name", o.ServiceName).String()
			},
		),
	)

	namespacedKubeInformers := informers.NewSharedInformerFactoryWithOptions(o.kubeClient, 12*time.Hour, informers.WithNamespace(o.Namespace))

	singleServiceInformer := singleServiceKubeInformers.Core().V1().Services()
	secretsInformer := namespacedKubeInformers.Core().V1().Secrets()

	member, err := identity.Retrieve(ctx, o.ServiceName, o.Namespace, o.kubeClient)
	if err != nil {
		return fmt.Errorf("can't get member info: %w", err)
	}

	// FIXME
	containerID := ""
	nodeName := ""

	var ncList *scyllav1alpha1.NodeConfigList
	err = wait.ExponentialBackoff(retry.DefaultBackoff, func() (bool, error) {
		ncList, err = o.scyllaClient.ScyllaV1alpha1().NodeConfigs().List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.ErrorS(err, "Can't list nodeconfigs")
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("can't list nodeconfigs: %w", err)
	}

	var node *corev1.Node
	err = wait.ExponentialBackoff(retry.DefaultBackoff, func() (bool, error) {
		node, err = o.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			klog.ErrorS(err, "Can't get node", "Name", nodeName)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("can't get node: %w", err)
	}

	// Filter nodeconfig selecting this node.
	var ncs []*scyllav1alpha1.NodeConfig
	for _, nc := range ncList.Items {
		isSelectingNode, err := helpers.IsNodeConfigSelectingNode(&nc, node)
		if err != nil {
			return fmt.Errorf(
				"can't check if nodecondig %q is selecting node %q: %w",
				naming.ObjRef(&nc),
				naming.ObjRef(node),
				err,
			)
		}
		if isSelectingNode {
			ncs = append(ncs, &nc)
		}
	}

	klog.V(2).InfoS("%d nodeconfig affect tuning on this node")

	// Wait for optimization status for each matching nodeconfig.
	for _, nc := range ncs {
		fieldSelector := fields.OneTermEqualSelector("metadata.name", nc.Name).String()
		lw := &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = fieldSelector
				return o.scyllaClient.ScyllaV1alpha1().NodeConfigs().List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = fieldSelector
				return o.scyllaClient.ScyllaV1alpha1().NodeConfigs().Watch(ctx, options)
			},
		}
		// TODO: extract into a function, use defer
		klog.V(2).InfoS("Waiting for optimizations", "NodeConfig", klog.KObj(nc))
		_, err := watchtools.UntilWithSync(
			ctx,
			lw,
			&scyllav1alpha1.NodeConfig{},
			nil,
			func(e watch.Event) (bool, error) {
				switch t := e.Type; t {
				case watch.Added, watch.Modified:
					return helpers.IsNodeTunedForContainer(nc, nodeName, containerID), nil
				case watch.Error:
					return true, apierrors.FromObject(e.Object)
				default:
					return true, fmt.Errorf("unexpected event type %v", t)
				}
			},
		)
		if err != nil {
			return fmt.Errorf("can't wait for optimization: %w", err)
		}
		klog.V(2).InfoS("Optimizations ready", "NodeConfig", klog.KObj(nc))
	}

	klog.V(2).InfoS("Starting scylla")

	cfg := config.NewScyllaConfig(member, o.kubeClient, o.scyllaClient, o.CPUCount)
	cmd, err := cfg.Setup(ctx)
	if err != nil {
		return fmt.Errorf("can't set up scylla: %w", err)
	}
	// Make sure to propagate the signal if we die.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	hostIP, err := network.FindFirstNonLocalIP()
	if err != nil {
		return fmt.Errorf("can't get node ip: %w", err)
	}

	hostAddr := hostIP.String()

	prober := sidecar.NewProber(
		o.Namespace,
		o.ServiceName,
		o.SecretName,
		singleServiceInformer.Lister(),
		secretsInformer.Lister(),
		hostAddr,
	)

	sc, err := sidecarcontroller.NewController(
		o.Namespace,
		o.ServiceName,
		o.SecretName,
		hostAddr,
		o.kubeClient,
		singleServiceInformer,
		secretsInformer,
	)
	if err != nil {
		return fmt.Errorf("can't create sidecar controller: %w", err)
	}

	// Start informers.
	singleServiceKubeInformers.Start(ctx.Done())
	namespacedKubeInformers.Start(ctx.Done())

	var wg sync.WaitGroup
	defer wg.Wait()

	// Run probes.
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", naming.ProbePort),
		Handler: nil,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()

		ok := cache.WaitForNamedCacheSync("Prober", ctx.Done(), secretsInformer.Informer().HasSynced)
		if !ok {
			return
		}

		klog.InfoS("Starting Prober server")
		defer klog.InfoS("Prober server shut down")

		http.HandleFunc(naming.LivenessProbePath, prober.Healthz)
		http.HandleFunc(naming.ReadinessProbePath, prober.Readyz)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.Fatal("ListenAndServe failed: %v", err)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		klog.InfoS("Shutting down Prober server")
		defer klog.InfoS("Shut down Prober server")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCtxCancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			klog.ErrorS(err, "Shutting down Prober server")
		}
	}()

	// Run sidecar controller.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc.Run(ctx)
	}()

	// Run scylla in a new process.
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("can't start scylla: %w", err)
	}

	defer func() {
		klog.InfoS("Waiting for scylla process to finish")
		defer klog.InfoS("Scylla process finished")

		err := cmd.Wait()
		if err != nil {
			klog.ErrorS(err, "Can't wait for scylla process to finish")
		}
	}()

	// Terminate the scylla process.
	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		klog.InfoS("Sending SIGTERM to the scylla process")
		err := cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			klog.ErrorS(err, "Can't send SIGTERM to the scylla process")
			return
		}
		klog.InfoS("Sent SIGTERM to the scylla process")
	}()

	<-ctx.Done()

	return nil
}
