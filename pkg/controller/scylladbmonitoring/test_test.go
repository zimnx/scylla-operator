package scylladbmonitoring

import (
	"testing"

	"k8s.io/klog/v2"
)

func makeFunc() func() {
	return func() {
		klog.InfoSDepth(1, "inside the helper func")
	}
}

func Test(t *testing.T) {
	klog.InfoS("foo")
	f := func() {
		makeFunc()()
	}

	f()
}
