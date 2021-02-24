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
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"

	apiv1 "github.com/PingCAP-QE/NaglfarCloud/pkg/api/v1"
)

// PodGroups implements PodGroupInterface
type PodGroups struct {
	client rest.Interface
	ns     string
}

// newPodGroups returns a PodGroups
func newPodGroups(c *SchedulingClient, namespace string) *PodGroups {
	return &PodGroups{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the podGroup, and returns the corresponding podGroup object, and an error if there is any.
func (c *PodGroups) Get(ctx context.Context, name string, options v1.GetOptions) (result *apiv1.PodGroup, err error) {
	result = &apiv1.PodGroup{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podgroups").
		Name(name).
		VersionedParams(&options, ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PodGroups that match those selectors.
func (c *PodGroups) List(ctx context.Context, opts v1.ListOptions) (result *apiv1.PodGroupList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &apiv1.PodGroupList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podgroups").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested podGroups.
func (c *PodGroups) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("podgroups").
		VersionedParams(&opts, ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a podGroup and creates it.  Returns the server's representation of the podGroup, and an error, if there is any.
func (c *PodGroups) Create(ctx context.Context, podGroup *apiv1.PodGroup, opts v1.CreateOptions) (result *apiv1.PodGroup, err error) {
	result = &apiv1.PodGroup{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("podgroups").
		VersionedParams(&opts, ParameterCodec).
		Body(podGroup).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a podGroup and updates it. Returns the server's representation of the podGroup, and an error, if there is any.
func (c *PodGroups) Update(ctx context.Context, podGroup *apiv1.PodGroup, opts v1.UpdateOptions) (result *apiv1.PodGroup, err error) {
	result = &apiv1.PodGroup{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podgroups").
		Name(podGroup.Name).
		VersionedParams(&opts, ParameterCodec).
		Body(podGroup).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *PodGroups) UpdateStatus(ctx context.Context, podGroup *apiv1.PodGroup, opts v1.UpdateOptions) (result *apiv1.PodGroup, err error) {
	result = &apiv1.PodGroup{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podgroups").
		Name(podGroup.Name).
		SubResource("status").
		VersionedParams(&opts, ParameterCodec).
		Body(podGroup).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the podGroup and deletes it. Returns an error if one occurs.
func (c *PodGroups) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podgroups").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *PodGroups) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podgroups").
		VersionedParams(&listOpts, ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched podGroup.
func (c *PodGroups) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *apiv1.PodGroup, err error) {
	result = &apiv1.PodGroup{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("podgroups").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
