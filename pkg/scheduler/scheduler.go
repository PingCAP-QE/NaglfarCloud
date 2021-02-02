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

package scheduler

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	schedulerRuntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
)

// 插件名称
const Name = "naglfar-scheduler"

var (
	_ framework.PreFilterPlugin = &Scheduler{}
	_ framework.FilterPlugin    = &Scheduler{}
	_ framework.PreBindPlugin   = &Scheduler{}
)

type Args struct{}

type Scheduler struct {
	args   *Args
	handle framework.Handle
}

func (s *Scheduler) Name() string {
	return Name
}

func (s *Scheduler) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) *framework.Status {
	klog.V(3).Infof("prefilter pod: %v", pod.Name)
	return framework.NewStatus(framework.Success, "")
}

func (s *Scheduler) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (s *Scheduler) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	klog.V(3).Infof("filter pod: %v, node: %+v", pod.Name, nodeInfo.Node())
	return framework.NewStatus(framework.Success, "")
}

func (s *Scheduler) PreBind(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	if nodeInfo, err := s.handle.SnapshotSharedLister().NodeInfos().Get(nodeName); err != nil {
		return framework.AsStatus(err)
	} else {
		klog.V(3).Infof("prebind node info: %+v", nodeInfo.Node())
		return framework.NewStatus(framework.Success, "")
	}
}

//type PluginFactory = func(configuration runtime.Object, f framework.Handle) (framework.Plugin, error)
func New(cfg runtime.Object, f framework.Handle) (framework.Plugin, error) {
	args := new(Args)

	if err := schedulerRuntime.DecodeInto(cfg, args); err != nil {
		return nil, err
	}

	klog.V(3).Infof("get plugin config args: %+v", args)
	return &Scheduler{
		args:   args,
		handle: f,
	}, nil
}
