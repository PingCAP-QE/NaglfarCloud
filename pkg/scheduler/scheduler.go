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
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	schedulerRuntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
)

// Name is the name of naglfar scheduler
const Name = "naglfar-scheduler"

var (
	_ framework.QueueSortPlugin  = &Scheduler{}
	_ framework.PreFilterPlugin  = &Scheduler{}
	_ framework.PostFilterPlugin = &Scheduler{}
	_ framework.PermitPlugin     = &Scheduler{}
	_ framework.ReservePlugin    = &Scheduler{}
	_ framework.PostBindPlugin   = &Scheduler{}
)

type Args struct {
	// ScheduleTimeout is the wait duration in scheduling
	ScheduleTimeout time.Duration `yaml:"scheduleTimeout" json:"scheduleTimeout"`
}

type Scheduler struct {
	args            *Args
	handle          framework.Handle
	podGroupManager *PodGroupManager
}

func (s *Scheduler) Name() string {
	return Name
}

func (s *Scheduler) Less(pod1, pod2 *framework.QueuedPodInfo) bool {
	return true
}

func (s *Scheduler) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) *framework.Status {
	// Do nothing
	return framework.NewStatus(framework.Success, "")
}

func (s *Scheduler) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (s *Scheduler) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	filteredNodeStatusMap framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	return nil, nil
}

func (s *Scheduler) Permit(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	return framework.NewStatus(framework.Success, ""), 0
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (s *Scheduler) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	return nil
}

// Unreserve rejects all other Pods in the PodGroup when one of the pods in the group times out.
func (s *Scheduler) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
}

// PostBind is called after a pod is successfully bound. These plugins are used update PodGroup when pod is bound.
func (s *Scheduler) PostBind(ctx context.Context, _ *framework.CycleState, pod *v1.Pod, nodeName string) {
}

// rejectPod rejects pod in cache
func (s *Scheduler) rejectPod(uid types.UID) {
	waitingPod := s.handle.GetWaitingPod(uid)
	if waitingPod == nil {
		return
	}
	waitingPod.Reject(Name)
}

//type PluginFactory = func(configuration runtime.Object, f framework.Handle) (framework.Plugin, error)
func New(cfg runtime.Object, f framework.Handle) (framework.Plugin, error) {
	args := new(Args)

	if err := schedulerRuntime.DecodeInto(cfg, args); err != nil {
		return nil, err
	}

	fieldSelector, err := fields.ParseSelector(",status.phase!=" + string(v1.PodSucceeded) + ",status.phase!=" + string(v1.PodFailed))
	if err != nil {
		klog.Fatalf("ParseSelector failed %+v", err)
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(f.ClientSet(), 0, informers.WithTweakListOptions(func(opt *metav1.ListOptions) {
		opt.LabelSelector = PodGroupLabel
		opt.FieldSelector = fieldSelector.String()
	}))

	klog.V(3).Infof("get plugin config args: %+v", args)
	return &Scheduler{
		args:            args,
		handle:          f,
		podGroupManager: NewPodGroupManager(f.SnapshotSharedLister(), args.ScheduleTimeout, informerFactory.Core().V1().Pods()),
	}, nil
}
