// Copyright (c) 2023 ScyllaDB.

package upgrade

import (
	"context"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
)

// Test is an interface for upgrade tests.
type Test interface {
	// Name returns a test name.
	Name() string

	// Setup creates and verifies whatever objects need to
	// exist before the upgrade disruption starts.
	Setup(ctx context.Context, f *framework.Framework)

	// Test runs during the upgrade. When the upgrade is
	// complete, done will be closed and final validation can
	// begin.
	Test(ctx context.Context, f *framework.Framework, done <-chan struct{})

	// Teardown cleans up any objects that are created that
	// aren't already cleaned up by the framework. This will
	// always be called, even if Setup failed.
	Teardown(ctx context.Context, f *framework.Framework)
}

type UpgradeFunc func(ctx context.Context) error

func RunUpgradeSuite(ctx context.Context, tests []Test, testFrameworks map[string]*framework.Framework, upgradeFunc UpgradeFunc) {
	stopCh := make(chan struct{})
	barriers := make([]*barrier, 0, len(tests))

	for i := range tests {
		tc := tests[i]

		b := &barrier{
			stop:  stopCh,
			ready: make(chan struct{}, 1),
			done:  make(chan struct{}, 1),
		}
		barriers = append(barriers, b)

		go func() {
			defer ginkgo.GinkgoRecover()
			defer b.Done()

			f := testFrameworks[tc.Name()]

			ginkgo.DeferCleanup(tc.Teardown, f)

			tc.Setup(ctx, f)
			b.Ready()
			tc.Test(ctx, f, b.stop)
		}()
	}

	framework.By("Waiting for all async tests to be ready")
	for _, sem := range barriers {
		sem.WaitForReadyOrDone()
	}

	defer func() {
		close(stopCh)
		framework.By("Waiting for async validations to complete")
		for _, sem := range barriers {
			sem.WaitForDone()
		}
	}()

	framework.By("Starting upgrade")
	err := upgradeFunc(ctx)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.By("Upgrade complete")
}

func CreateUpgradeFrameworks(tests []Test) map[string]*framework.Framework {
	fs := make(map[string]*framework.Framework, len(tests))
	for _, tc := range tests {
		// match anything that's not a word character or hyphen
		// and replace with a single hyphen.
		nsRe := regexp.MustCompile("[^[:word:]-]+")
		ns := nsRe.ReplaceAllString(tc.Name(), "-")
		ns = strings.Trim(ns, "-")
		ns = strings.ToLower(ns)
		fs[tc.Name()] = framework.NewFramework(ns)
	}

	return fs
}

type barrier struct {
	ready chan struct{}
	stop  chan struct{}
	done  chan struct{}
}

func (s *barrier) Ready() {
	close(s.ready)
}

func (s *barrier) Done() {
	close(s.done)
}

func (s *barrier) WaitForReadyOrDone() {
	select {
	case <-s.ready:
	case <-s.done:
	}
}

func (s *barrier) WaitForDone() {
	<-s.done
}
