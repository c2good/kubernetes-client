/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package nodeenv

import (
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/util/labelselector"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("OriginPodNodeEnvironment",
		func(config io.Reader) (admission.Interface, error) {
			return NewPodNodeEnvironment()
		})
}

// podNodeEnvironment is an implementation of admission.Interface.
type podNodeEnvironment struct {
	*admission.Handler
	client kclientset.Interface
	cache  *cache.ProjectCache
}

var _ = oadmission.WantsProjectCache(&podNodeEnvironment{})
var _ = kadmission.WantsInternalKubeClientSet(&podNodeEnvironment{})

// Admit enforces that pod and its project node label selectors matches at least a node in the cluster.
func (p *podNodeEnvironment) Admit(a admission.Attributes) (err error) {
	resource := a.GetResource().GroupResource()
	if resource != kapi.Resource("pods") {
		return nil
	}
	if a.GetSubresource() != "" {
		// only run the checks below on pods proper and not subresources
		return nil
	}

	obj := a.GetObject()
	pod, ok := obj.(*kapi.Pod)
	if !ok {
		return nil
	}

	name := pod.Name

	if !p.cache.Running() {
		return err
	}
	namespace, err := p.cache.GetNamespace(a.GetNamespace())
	if err != nil {
		return apierrors.NewForbidden(resource, name, err)
	}
	projectNodeSelector, err := p.cache.GetNodeSelectorMap(namespace)
	if err != nil {
		return err
	}

	if labelselector.Conflicts(projectNodeSelector, pod.Spec.NodeSelector) {
		return apierrors.NewForbidden(resource, name, fmt.Errorf("pod node label selector conflicts with its project node label selector"))
	}

	// modify pod node selector = project node selector + current pod node selector
	pod.Spec.NodeSelector = labelselector.Merge(projectNodeSelector, pod.Spec.NodeSelector)

	return nil
}

func (p *podNodeEnvironment) SetProjectCache(c *cache.ProjectCache) {
	p.cache = c
}

func (q *podNodeEnvironment) SetInternalKubeClientSet(c kclientset.Interface) {
	q.client = c
}

func (p *podNodeEnvironment) ValidateInitialization() error {
	if p.cache == nil {
		return fmt.Errorf("project node environment plugin needs a project cache")
	}
	return nil
}

func NewPodNodeEnvironment() (admission.Interface, error) {
	return &podNodeEnvironment{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}
