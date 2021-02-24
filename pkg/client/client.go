// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	rest "k8s.io/client-go/rest"

	apiv1 "github.com/PingCAP-QE/NaglfarCloud/pkg/api/v1"
)

var Scheme = runtime.NewScheme()
var Codecs = serializer.NewCodecFactory(Scheme)
var ParameterCodec = runtime.NewParameterCodec(Scheme)

// SchedulingClient is used to interact with features provided by the scheduling.sigs.k8s.io group.
type SchedulingClient struct {
	restClient rest.Interface
}

func (c *SchedulingClient) PodGroups(namespace string) *PodGroups {
	return newPodGroups(c, namespace)
}

// NewForConfig creates a new SchedulingClient for the given config.
func NewForConfig(c *rest.Config) (*SchedulingClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &SchedulingClient{client}, nil
}

// NewForConfigOrDie creates a new SchedulingV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *SchedulingClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SchedulingClient for the given RESTClient.
func New(c rest.Interface) *SchedulingClient {
	return &SchedulingClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	config.GroupVersion = &apiv1.GroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *SchedulingClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}

func init() {
	v1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(apiv1.AddToScheme(Scheme))
}
