// Copyright (C) 2021 ScyllaDB

package framework

import (
	restclient "k8s.io/client-go/rest"
)

var TestContext *TestContextType

type DeleteTestingNSPolicyType string

var (
	DeleteTestingNSPolicyAlways    DeleteTestingNSPolicyType = "Always"
	DeleteTestingNSPolicyOnSuccess DeleteTestingNSPolicyType = "OnSuccess"
	DeleteTestingNSPolicyNever     DeleteTestingNSPolicyType = "Never"
)

type IngressController struct {
	IngressClassName  string
	Address           string
	CustomAnnotations map[string]string
}

type TestContextType struct {
	RestConfig            *restclient.Config
	ArtifactsDir          string
	DeleteTestingNSPolicy DeleteTestingNSPolicyType
	IngressController     IngressController
}
