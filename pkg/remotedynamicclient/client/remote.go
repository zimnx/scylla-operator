// Copyright (c) 2022 ScyllaDB.

package client

import (
	"crypto/sha512"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type RemoteInterface interface {
	Region(region string) (dynamic.Interface, error)
}

type Dynamic interface {
	Update(region string, config []byte) error
	Delete(region string)
}

type RemoteDynamicClient struct {
	lock             sync.RWMutex
	localClient      dynamic.Interface
	credentialHashes map[string]string
	regionClients    map[string]dynamic.Interface
}

func hash(obj []byte) string {
	return string(sha512.New().Sum(obj))
}

func NewRemoteDynamicClient() *RemoteDynamicClient {
	rc := &RemoteDynamicClient{
		lock:             sync.RWMutex{},
		credentialHashes: make(map[string]string),
		regionClients:    make(map[string]dynamic.Interface),
	}

	return rc
}

var emptyHash = hash(nil)

func (c *RemoteDynamicClient) Update(region string, config []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	credentialsHash := hash(config)
	if h, ok := c.credentialHashes[region]; ok && h == credentialsHash {
		return nil
	}

	if equality.Semantic.DeepEqual(credentialsHash, emptyHash) {
		c.regionClients[region] = c.localClient
		c.credentialHashes[region] = credentialsHash
		return nil
	}

	klog.V(4).InfoS("Updating remote client", "region", region)

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(config)
	if err != nil {
		return fmt.Errorf("can't create REST config from kubeconfig: %w", err)
	}

	dc, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("can't create client: %w", err)
	}

	c.regionClients[region] = dc
	c.credentialHashes[region] = credentialsHash

	return nil
}

func (c *RemoteDynamicClient) Delete(region string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	klog.V(4).InfoS("Removing remote client", "region", region)
	delete(c.regionClients, region)
	delete(c.credentialHashes, region)
}

func (c *RemoteDynamicClient) Region(region string) (dynamic.Interface, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	client, ok := c.regionClients[region]
	if !ok {
		return nil, fmt.Errorf("client for %q region not found", region)
	}
	return client, nil
}
