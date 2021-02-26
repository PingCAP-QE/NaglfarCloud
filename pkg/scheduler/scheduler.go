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
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	schedulerRuntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"

	"github.com/PingCAP-QE/NaglfarCloud/pkg/client"
	"github.com/PingCAP-QE/NaglfarCloud/pkg/util"
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
	ScheduleTimeout util.Duration `yaml:"scheduleTimeout" json:"scheduleTimeout"`
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
	time1 := s.podGroupManager.GetCreationTimestamp(pod1.Pod, pod1.InitialAttemptTimestamp)
	time2 := s.podGroupManager.GetCreationTimestamp(pod2.Pod, pod2.InitialAttemptTimestamp)

	if time1.Equal(time2) {
		return pod1.Pod.Labels[PodGroupLabel] < pod2.Pod.Labels[PodGroupLabel]
	}

	return time1.Before(time2)
}

func (s *Scheduler) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) *framework.Status {
	// Do nothing
	pg, err := s.podGroupManager.PodGroup(pod)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}

	if pg != nil {
		klog.V(3).Infof("podgroup: %v", pg)
	}
	return framework.NewStatus(framework.Success, "")
}

func (s *Scheduler) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (s *Scheduler) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	filteredNodeStatusMap framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	return nil, nil
}

// Permit is the functions invoked by the framework at "Permit" extension point.
func (s *Scheduler) Permit(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	waitTime := s.args.ScheduleTimeout.Unwrap()
	ready, podGroup, err := s.podGroupManager.Permit(ctx, pod, nodeName)
	if err != nil {
		if podGroup == nil {
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, "PodGroup not found"), waitTime
		}
		if err == ErrorWaiting {
			klog.Infof("Pod: %s/%s is waiting to be scheduled to node: %v", pod.Namespace, pod.Name, nodeName)
			return framework.NewStatus(framework.Wait, ""), waitTime
		}
		klog.Infof("Permit error %v", err)
		return framework.NewStatus(framework.Unschedulable, err.Error()), waitTime
	}

	klog.V(5).Infof("Pod requires podgroup %s", pod.Labels[PodGroupLabel])
	if !ready {
		return framework.NewStatus(framework.Wait, ""), waitTime
	}

	s.handle.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		pod := waitingPod.GetPod()
		if pod.Namespace == podGroup.Namespace && pod.Labels[PodGroupLabel] == podGroup.Name {
			klog.V(3).Infof("Permit allows the pod: %s/%s", pod.Namespace, pod.Name)
			waitingPod.Allow(s.Name())
		}
	})

	klog.V(3).Infof("Permit allows the pod: %s/%s", pod.Namespace, pod.Name)
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

	if _, err := args.ScheduleTimeout.Parse(); err != nil {
		return nil, fmt.Errorf("invalid ScheduleTimeout: %s", err.Error())
	}

	conf, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to init rest.Config: %v", err)
	}

	schedulingClient := client.NewForConfigOrDie(conf)

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
		podGroupManager: NewPodGroupManager(f.SnapshotSharedLister(), args.ScheduleTimeout.Unwrap(), informerFactory.Core().V1().Pods(), schedulingClient),
	}, nil
}
