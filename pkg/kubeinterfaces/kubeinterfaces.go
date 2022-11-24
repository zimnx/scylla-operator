package kubeinterfaces

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ObjectInterface interface {
	runtime.Object
	metav1.Object
}
