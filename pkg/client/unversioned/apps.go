/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unversioned

import (
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type AppsInterface interface {
	StatefulSetNamespacer
}

// AppsClient is used to interact with Kubernetes batch features.
type AppsClient struct {
	*restclient.RESTClient
}

func (c *AppsClient) StatefulSets(namespace string) StatefulSetInterface {
	return newStatefulSet(c, namespace)
}

func NewApps(c *restclient.Config) (*AppsClient, error) {
	config := *c
	if err := setGroupDefaults(apps.GroupName, &config); err != nil {
		return nil, err
	}
	client, err := restclient.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &AppsClient{client}, nil
}

func NewAppsOrDie(c *restclient.Config) *AppsClient {
	client, err := NewApps(c)
	if err != nil {
		panic(err)
	}
	return client
}
