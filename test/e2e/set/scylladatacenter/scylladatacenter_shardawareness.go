// Copyright (C) 2021 ScyllaDB

package scylladatacenter

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/gocqlx/v2"
	scyllaclusterfixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
)

var _ = g.Describe("ScyllaDatacenter", func() {
	defer g.GinkgoRecover()

	f := framework.NewFramework("scylladatacenter")

	g.It("should allow to build connection pool using shard aware ports", func() {
		g.Skip("Shardawareness doesn't work on setups NATting traffic, and our CI does it when traffic is going through ClusterIPs." +
			"It's shall be reenabled once we switch client-node communication to PodIPs.",
		)

		const (
			nonShardAwarePort = 9042
			shardAwarePort    = 19042
			nrShards          = 2
		)

		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sd := scyllaclusterfixture.BasicScyllaDatacenter.ReadOrFail()
		sd.Spec.Racks[0].Nodes = pointer.Int32(1)

		// Ensure 2 shards.
		sd.Spec.Racks[0].Scylla.Resources.Limits[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%d", nrShards))

		framework.By("Creating a ScyllaDatacenter")
		sd, err := f.ScyllaClient().ScyllaV1alpha1().ScyllaDatacenters(f.Namespace()).Create(ctx, sd, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaDatacenter to rollout (RV=%s)", sd.ResourceVersion)
		waitCtx1, waitCtx1Cancel := v1alpha1.ContextForRollout(ctx, sd)
		defer waitCtx1Cancel()
		sd, err = v1alpha1.WaitForScyllaDatacenterState(waitCtx1, f.ScyllaClient().ScyllaV1alpha1(), sd.Namespace, sd.Name, v1alpha1.WaitForStateOptions{}, v1alpha1.IsScyllaDatacenterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		hosts, err := v1alpha1.GetHosts(ctx, f.KubeClient().CoreV1(), sd)
		o.Expect(err).NotTo(o.HaveOccurred())

		connections := make(map[uint16]string)
		var connectionsMut sync.Mutex

		clusterConfig := gocql.NewCluster(hosts...)
		clusterConfig.Dialer = DialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
			sourcePort := gocql.ScyllaGetSourcePort(ctx)
			localAddr, err := net.ResolveTCPAddr(network, fmt.Sprintf(":%d", sourcePort))
			if err != nil {
				return nil, err
			}

			framework.Infof("Connecting to %s using %d source port", addr, sourcePort)
			connectionsMut.Lock()
			connections[sourcePort] = addr
			connectionsMut.Unlock()

			d := &net.Dialer{LocalAddr: localAddr}
			return d.DialContext(ctx, network, addr)
		})

		framework.By("Waiting for the driver to establish connection to shards")
		session, err := gocqlx.WrapSession(clusterConfig.CreateSession())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer session.Close()

		err = wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (done bool, err error) {
			return len(connections) == nrShards, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		shardAwareAttempts := 0
		for sourcePort, addr := range connections {
			// Control connection is also put in pool, and it always uses the default port.
			if sourcePort == 0 {
				o.Expect(addr).To(o.HaveSuffix(fmt.Sprintf("%d", nonShardAwarePort)))
				continue
			}
			o.Expect(addr).To(o.HaveSuffix(fmt.Sprintf("%d", shardAwarePort)))
			shardAwareAttempts++
		}

		// Control connection used for shard number discovery, lands on some random shard.
		// This connection is also put in pool, and driver only establish connections to missing shards
		// using shard-aware-port.
		// Connections to shard-aware-port are guaranteed to land on shard driver wants.
		o.Expect(shardAwareAttempts).To(o.Equal(nrShards - 1))
	})
})

type DialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func (f DialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}
