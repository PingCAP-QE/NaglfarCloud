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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	schedulerRuntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"

	util "github.com/PingCAP-QE/NaglfarCloud/pkg/api/v1"
	"github.com/PingCAP-QE/NaglfarCloud/pkg/client"
)

// Name is the name of naglfar scheduler
const (
	Name = "naglfar-scheduler"
)

var (
	_    framework.QueueSortPlugin  = &Scheduler{}
	_    framework.PreFilterPlugin  = &Scheduler{}
	_    framework.FilterPlugin     = &Scheduler{}
	_    framework.PostFilterPlugin = &Scheduler{}
	_    framework.ScorePlugin      = &Scheduler{}
	_    framework.PermitPlugin     = &Scheduler{}
	_    framework.ReservePlugin    = &Scheduler{}
	_    framework.PostBindPlugin   = &Scheduler{}
	noOp                            = NoOp{}
)

// Args is the arguements for initializing scheduler
type Args struct {
	// KubeConfig is the kubeconfig file path
	KubeConfig string `yaml:"kubeconfig" json:"kubeconfig"`
	// ScheduleTimeout is the wait duration in scheduling
	ScheduleTimeout util.Duration `yaml:"scheduleTimeout" json:"scheduleTimeout"`
	// RescheduleDelayOffset is the shift of the next reschedule time since now
	RescheduleDelayOffset util.Duration `yaml:"rescheduleDelayOffset" json:"rescheduleDelayOffset"`
}

// Scheduler is the custom scheduler of naglfar system
type Scheduler struct {
	args            *Args
	handle          framework.FrameworkHandle
	podGroupManager *PodGroupManager
}

// Name is the name of scheduler
func (s *Scheduler) Name() string {
	return Name
}

// Less are used to sort pods in the scheduling queue.
func (s *Scheduler) Less(podInfo1, podInfo2 *framework.QueuedPodInfo) bool {
	time1 := s.podGroupManager.getScheduleTime(podInfo1.Pod, podInfo1.Timestamp)
	time2 := s.podGroupManager.getScheduleTime(podInfo2.Pod, podInfo2.Timestamp)

	if time1.Equal(time2) {
		return podInfo1.Pod.Labels[PodGroupLabel] < podInfo2.Pod.Labels[PodGroupLabel]
	}

	return time1.Before(time2)
}

// PreFilter is called at the beginning of the scheduling cycle. All PreFilter
// plugins must return success or the pod will be rejected.
func (s *Scheduler) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) *framework.Status {
	// If any validation failed, a no-op state data is injected to "state" so that in later
	// phases we can tell whether the failure comes from PreFilter or not.
	if err := s.podGroupManager.PreFilter(ctx, pod); err != nil {
		klog.Error(err)
		state.Write(s.getStateKey(), noOp)
		return framework.NewStatus(framework.Unschedulable, err.Error())
	}

	return framework.NewStatus(framework.Success, "")
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one,
// or nil if it does not. A Pre-filter plugin can provide extensions to incrementally
// modify its pre-processed info. The framework guarantees that the extensions
// AddPod/RemovePod will only be called after PreFilter, possibly on a cloned
// CycleState, and may call those functions more than once before calling
// Filter again on a specific node.
func (s *Scheduler) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Filter is called by the scheduling framework.
// All FilterPlugins should return "Success" to declare that
// the given node fits the pod. If Filter doesn't return "Success",
// it will return "Unschedulable", "UnschedulableAndUnresolvable" or "Error".
// For the node being evaluated, Filter plugins should look at the passed
// nodeInfo reference for this particular node's information (e.g., pods
// considered to be running on the node) instead of looking it up in the
// NodeInfoSnapshot because we don't guarantee that they will be the same.
// For example, during preemption, we may pass a copy of the original
// nodeInfo object that has some pods removed from it to evaluate the
// possibility of preempting them to schedule the target pod.
func (s *Scheduler) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	if isControlledByDaemonSet(pod) {
		return framework.NewStatus(framework.Success, "")
	}

	pg, pgSpec, err := s.podGroupManager.podGroup(pod)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("cannot get pod group of %s/%s: %s", pod.Namespace, pod.Name, err.Error()))
	}

	for _, podInfo := range nodeInfo.Pods {
		if isControlledByDaemonSet(podInfo.Pod) {
			continue
		}
		pipg, pipgSpec, err := s.podGroupManager.podGroup(podInfo.Pod)
		if err != nil {
			return framework.NewStatus(framework.Error, fmt.Sprintf("cannot get pod group of %s/%s: %s", podInfo.Pod.Namespace, podInfo.Pod.Name, err.Error()))
		}
		switch {
		// can't monopolized a non-exclusive node
		case pgSpec != nil && pgSpec.IsExclusive() && !isNodeAnnotatedExclusive(nodeInfo):
			return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("non-exclusive node %s cannot be monopolized by podgroup %s/%s", nodeInfo.Node().Name, pg.Namespace, pg.Name))
		// can't monopolized a exclusive node since there are some non-podGroup pods on that
		case pgSpec != nil && pgSpec.IsExclusive() && pipgSpec == nil:
			return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("node %s cannot be monopolized by podgroup %s/%s", nodeInfo.Node().Name, pg.Namespace, pg.Name))
		case pgSpec == nil:
			// can't schedule this pod on the node which has been monopolized
			if pipgSpec != nil && pipgSpec.IsExclusive() {
				return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("node %s is already monopolized by podgroup %s/%s", nodeInfo.Node().Name, pipg.Namespace, pipg.Name))
			}
			// can't schedule this pod on the node since it can't tolerate the exclusive node
			if isNodeAnnotatedExclusive(nodeInfo) && !canPodTolerateExclusiveNode(pod) {
				return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("node %s is annotated with naglfar/exclusive but pod %s/%s can't tolerate it", nodeInfo.Node().Name, pod.Namespace, pod.Name))
			}
		// can't schedule a node since there are some mutually exclusive pods on that
		case pg != nil && pipg != nil && pg.UID != pipg.UID:
			if pgSpec.IsExclusive() || pipgSpec.IsExclusive() {
				return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("node %s cannot schedule %s/%s[exclusive=%v] since a mutually exclusive podGroup %s/%s[exclusive=%v] has already scheduled on it", nodeInfo.Node().Name, pg.Namespace, pg.Name, pgSpec.IsExclusive(), pipg.Namespace, pipg.Name, pipgSpec.IsExclusive()))
			}
		}
	}
	return framework.NewStatus(framework.Success, "")
}

// PostFilter is called by the scheduling framework.
// A PostFilter plugin should return one of the following statuses:
// - Unschedulable: the plugin gets executed successfully but the pod cannot be made schedulable.
// - Success: the plugin gets executed successfully and the pod can be made schedulable.
// - Error: the plugin aborts due to some internal error.
//
// Informational plugins should be configured ahead of other ones, and always return Unschedulable status.
// Optionally, a non-nil PostFilterResult may be returned along with a Success status. For example,
// a preemption plugin may choose to return nominatedNodeName, so that framework can reuse that to update the
// preemptor pod's .spec.status.nominatedNodeName field.
//
// Is used to rejecting a group of pods if a pod does not pass PreFilter or Filter.
func (s *Scheduler) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	filteredNodeStatusMap framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	pg, pgSpec, err := s.podGroupManager.podGroup(pod)
	if err != nil {
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Error, err.Error())
	}
	if pg == nil {
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable, "no binding pod group")
	}
	pgn := getPodGroupNameFromPod(pod)
	// Check if the failure comes from PreFilter.
	op, _ := state.Read(s.getStateKey())
	if op == noOp {
		state.Delete(s.getStateKey())
		if err := s.podGroupManager.reschedule(pg, pgn, s.args.RescheduleDelayOffset.Unwrap()); err != nil {
			klog.Errorf("Failed to reschedule pod group %s/%s: %s", pg.Namespace, pg.Name, err.Error())
		}
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable)
	}
	// This indicates there are already enough Pods satisfying the PodGroup,
	// so don't bother to reject the whole PodGroup.
	assigned := s.podGroupManager.calculateAssignedPods(pod.Namespace, getPodGroupNameSliceFromPod(pod))
	if assigned >= int(pgSpec.MinMember) {
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable)
	}

	if err := s.podGroupManager.reschedule(pg, pgn, s.args.RescheduleDelayOffset.Unwrap()); err != nil {
		klog.Errorf("Failed to reschedule pod group %s/%s: %s", pg.Namespace, pg.Name, err.Error())
	}
	// It's based on an implicit assumption: if the nth Pod failed,
	// it's inferrable other Pods belonging to the same PodGroup would be very likely to fail.
	s.handle.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		if waitingPod.GetPod().Namespace == pod.Namespace && getPodGroupNameFromPod(waitingPod.GetPod()) == getPodGroupNameFromPod(pod) {
			klog.Infof("PostFilter rejects the pod: %s/%s", pod.Namespace, waitingPod.GetPod().Name)
			waitingPod.Reject(s.Name())
		}
	})

	return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable,
		fmt.Sprintf("PodGroup %v gets rejected due to Pod %s/%s is unschedulable even after PostFilter", getPodGroupNameFromPod(pod), pod.Namespace, pod.Name))
}

// Score is called on each filtered node. It must return success and an integer
// indicating the rank of the node. All scoring plugins must return success or
// the pod will be rejected.
func (s *Scheduler) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	podGroup, _, err := s.podGroupManager.podGroup(pod)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("cannot get pod group: %s", err.Error()))
	}

	if podGroup != nil {
		node, err := s.podGroupManager.snapshotSharedLister.NodeInfos().Get(nodeName)
		if err != nil {
			klog.Errorf("Cannot get nodeInfos from frameworkHandle: %v", err)
			return 0, framework.NewStatus(framework.Error, err.Error())
		}

		for _, friend := range node.Pods {
			names := getPodGroupNameSliceFromPod(friend.Pod)
			if friend.Pod.Namespace == podGroup.Namespace && !names.IsEmpty() && names[0] == podGroup.Name {
				return 100, framework.NewStatus(framework.Success, "")
			}
		}
	}

	return 0, framework.NewStatus(framework.Success, "")
}

// ScoreExtensions returns a ScoreExtensions interface if it implements one, or nil if does not.
func (s *Scheduler) ScoreExtensions() framework.ScoreExtensions {
	return nil
}

// Permit is the functions invoked by the framework at "Permit" extension point.
func (s *Scheduler) Permit(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	pgName := getPodGroupNameFromPod(pod)
	if pgName == "" {
		return framework.NewStatus(framework.Success, ""), 0
	}
	ready, waitTime, err := s.podGroupManager.Permit(ctx, pod, nodeName, s.args.ScheduleTimeout.Unwrap())
	if err != nil {
		if err == errorWaiting {
			klog.Infof("Pod: %s/%s is waiting to be scheduled to node: %v", pod.Namespace, pod.Name, nodeName)
			return framework.NewStatus(framework.Wait, ""), waitTime
		}
		klog.Errorf("Permit error %s", err.Error())
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error()), waitTime
	}

	if !ready {
		return framework.NewStatus(framework.Wait, ""), waitTime
	}

	namespace := pod.Namespace
	s.handle.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		pod := waitingPod.GetPod()
		if pod.Namespace == namespace && getPodGroupNameFromPod(pod) == pgName {
			klog.Infof("Permit allows the pod: %s/%s", pod.Namespace, pod.Name)
			waitingPod.Allow(s.Name())
		}
	})

	klog.Infof("Permit allows the pod: %s/%s", pod.Namespace, pod.Name)
	return framework.NewStatus(framework.Success, ""), 0
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (s *Scheduler) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	// do nothing
	return nil
}

// Unreserve rejects all other Pods in the PodGroup when one of the pods in the group times out.
func (s *Scheduler) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	podGroup, _, err := s.podGroupManager.podGroup(pod)
	if err != nil {
		klog.Errorf("Cannot get pod group: %s", err.Error())
		return
	}
	if podGroup != nil {
		if err := s.podGroupManager.reschedule(podGroup, getPodGroupNameFromPod(pod), s.args.RescheduleDelayOffset.Unwrap()); err != nil {
			klog.Errorf("Failed to reschedule pod group %s/%s: %s", podGroup.Namespace, podGroup.Name, err.Error())
		}
		s.handle.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
			if waitingPod.GetPod().Namespace == pod.Namespace && getPodGroupNameFromPod(waitingPod.GetPod()) == getPodGroupNameFromPod(pod) {
				klog.V(3).Infof("Unreserve rejects the pod: %s/%s", waitingPod.GetPod().Namespace, waitingPod.GetPod().Name)
				waitingPod.Reject(s.Name())
			}
		})
	}
}

// PostBind is called after a pod is successfully bound. These plugins are used update PodGroup when pod is bound.
func (s *Scheduler) PostBind(ctx context.Context, _ *framework.CycleState, pod *v1.Pod, nodeName string) {
	// do nothing
}

func (s *Scheduler) getStateKey() framework.StateKey {
	return framework.StateKey(fmt.Sprintf("Prefilter-%v", s.Name()))
}

// New is the constructor of Scheduler
func New(cfg runtime.Object, f framework.FrameworkHandle) (framework.Plugin, error) {
	args := new(Args)
	if err := schedulerRuntime.DecodeInto(cfg, args); err != nil {
		return nil, err
	}
	if _, err := args.ScheduleTimeout.Parse(); err != nil {
		return nil, fmt.Errorf("invalid ScheduleTimeout: %s", err.Error())
	}
	if _, err := args.RescheduleDelayOffset.Parse(); err != nil {
		return nil, fmt.Errorf("invalid rescheduleDelayOffset: %s", err.Error())
	}
	klog.V(3).Infof("Get plugin config args: %+v", args)

	conf, err := clientcmd.BuildConfigFromFlags("", args.KubeConfig)
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
	podInformer := informerFactory.Core().V1().Pods()
	ctx := context.TODO()
	plugin := &Scheduler{
		args:            args,
		handle:          f,
		podGroupManager: NewPodGroupManager(f.SnapshotSharedLister(), args.ScheduleTimeout.Unwrap(), podInformer, f.ClientSet(), schedulingClient),
	}

	informerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.Informer().HasSynced) {
		klog.Error("Cannot sync caches")
		return nil, fmt.Errorf("WaitForCacheSync failed")
	}
	return plugin, nil
}

func isNodeAnnotatedExclusive(nodeInfo *framework.NodeInfo) bool {
	exclusive, exist := nodeInfo.Node().Annotations[exclusiveNodeAnnotation]
	return exist && exclusive == "true"
}

func canPodTolerateExclusiveNode(pod *v1.Pod) bool {
	edn, exist := pod.Annotations[estimationDurationAnnotation]
	return exist && tolerateEstimationDurationValue == edn
}
