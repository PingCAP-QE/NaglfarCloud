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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	informerv1 "k8s.io/client-go/informers/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"

	apiv1 "github.com/PingCAP-QE/NaglfarCloud/pkg/api/v1"
	"github.com/PingCAP-QE/NaglfarCloud/pkg/client"
)

// PodGroupLabel is the default label of naglfar scheduler
const PodGroupLabel = "podgroup.naglfar"
const subGroupSeparator = "."

var errorWaiting = fmt.Errorf("waiting")

func subGroupNotFound(podGroup *apiv1.PodGroup, name string) error {
	return fmt.Errorf("subgroup %s not found in group %s/%s", name, podGroup.Namespace, podGroup.Name)
}

// PodGroupManager is the mananger of podgroup
type PodGroupManager struct {
	ctx context.Context
	// snapshotSharedLister is pod shared list
	snapshotSharedLister framework.SharedLister

	// scheduleTimeout is the default time when group scheduling.
	// If podgroup's ScheduleTimeoutSeconds set, that would be used.
	scheduleTimeout time.Duration

	// podLister is pod lister
	podLister listerv1.PodLister

	schedulingClient *client.SchedulingClient
}

// NewPodGroupManager is the constructor of PodGroupManager
func NewPodGroupManager(snapshotSharedLister framework.SharedLister, scheduleTimeout time.Duration, podInformer informerv1.PodInformer, schedulingClient *client.SchedulingClient) *PodGroupManager {
	return &PodGroupManager{
		ctx:                  context.Background(),
		snapshotSharedLister: snapshotSharedLister,
		scheduleTimeout:      scheduleTimeout,
		podLister:            podInformer.Lister(),
		schedulingClient:     schedulingClient,
	}
}

// PreFilter filters out a pod if it
// - Check if the numbers of total pods is less than minMenber
func (mgr *PodGroupManager) PreFilter(ctx context.Context, pod *corev1.Pod) error {
	super, sub, err := mgr.podGroups(pod)
	if err != nil {
		return fmt.Errorf("cannot get pod group: %s", err.Error())
	}

	if super == nil {
		return nil
	}

	friends, err := mgr.podLister.List(newSubGroupSelector(groupNames(pod)))
	if err != nil {
		return fmt.Errorf("cannot list pods in the same group: %s", err.Error())
	}

	if len(friends) < int(sub.MinMember) {
		return fmt.Errorf("the numbers of pods in the same group is less than minMember: %d < %d", len(friends), sub.MinMember)
	}
	return nil
}

// Permit permits a pod to run
func (mgr *PodGroupManager) Permit(ctx context.Context, pod *corev1.Pod, nodeName string) (bool, error) {
	podGroup, subPodGroup, err := mgr.podGroups(pod)

	if err != nil {
		return false, fmt.Errorf("cannot get pod group: %v", err)
	}

	if podGroup == nil {
		return true, nil
	}

	assigned := mgr.calculateAssignedPods(pod)
	// The number of pods that have been assigned nodes is calculated from the snapshot.
	// The current pod in not included in the snapshot during the current scheduling cycle.
	if assigned+1 < int(subPodGroup.MinMember) {
		return false, errorWaiting
	}
	return true, nil
}

// podGroups returns the super pod group and sub pod group that a Pod belongs to.
func (mgr *PodGroupManager) podGroups(pod *corev1.Pod) (*apiv1.PodGroup, *apiv1.PodGroupSpec, error) {
	names := groupNames(pod)
	if len(names) == 0 {
		return nil, nil, nil
	}

	podGroup, err := mgr.schedulingClient.PodGroups(pod.Namespace).Get(mgr.ctx, names[0], metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	subPodGroup := &podGroup.Spec
	for _, name := range names[1:] {
		if subPodGroup.SubGroups == nil {
			return nil, nil, subGroupNotFound(podGroup, name)
		}
		spec, ok := subPodGroup.SubGroups[name]
		if !ok {
			return nil, nil, subGroupNotFound(podGroup, name)
		}

		if spec.Exclusive == nil && subPodGroup.Exclusive != nil {
			// inherit exclusive from super group
			spec.Exclusive = &*subPodGroup.Exclusive
		}
		subPodGroup = &spec
	}

	return podGroup, subPodGroup, nil
}

func (mgr *PodGroupManager) calculateAssignedPods(target *corev1.Pod) int {
	nodeInfos, err := mgr.snapshotSharedLister.NodeInfos().List()
	if err != nil {
		klog.Errorf("Cannot get nodeInfos from frameworkHandle: %v", err)
		return 0
	}
	var count int
	for _, nodeInfo := range nodeInfos {
		for _, podInfo := range nodeInfo.Pods {
			pod := podInfo.Pod
			prefixNames := groupNames(target)
			names := groupNames(pod)
			if len(names) >= len(prefixNames) &&
				joinNames(names[0:len(prefixNames)]) == joinNames(prefixNames) &&
				pod.Namespace == target.Namespace &&
				pod.Spec.NodeName != "" {
				count++
			}
		}
	}

	return count
}

// rescheduleAfterAll is a method to reschedule pod group to end of queue.
// the duration is the delay of current last pog group.
func (mgr *PodGroupManager) rescheduleAfterAll(podGroup *apiv1.PodGroup, dur time.Duration) (*apiv1.PodGroup, error) {
	podGroups, err := mgr.schedulingClient.PodGroups(podGroup.Namespace).List(mgr.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("fail to list pod group: %s", err.Error())
	}

	maxTime := time.Time{}
	for _, pg := range podGroups.Items {
		if pg.SchedulingTime().After(maxTime) {
			maxTime = pg.SchedulingTime()
		}
	}

	podGroup.Status.NextSchedulingTime.Time = maxTime.Add(dur)
	return mgr.schedulingClient.PodGroups(podGroup.Namespace).UpdateStatus(mgr.ctx, podGroup, metav1.UpdateOptions{})
}

func (mgr *PodGroupManager) getSchedulingTime(pod *corev1.Pod, defaultTime time.Time) time.Time {
	podGroup, _, _ := mgr.podGroups(pod)
	if podGroup == nil {
		return defaultTime
	}

	return podGroup.SchedulingTime()
}

// groupPath is a function to get podgroup label of pod
func groupPath(pod *corev1.Pod) string {
	return strings.TrimSpace(pod.Labels[PodGroupLabel])
}

// groupNames is a function to split podgroup by slash
// return nil if the podgroup label is not set
func groupNames(pod *corev1.Pod) []string {
	path := groupPath(pod)
	if path == "" {
		return nil
	}
	return strings.Split(path, subGroupSeparator)
}

func joinNames(names []string) string {
	return strings.Join(names, subGroupSeparator)
}

func isControlledByDaemon(pod *corev1.Pod) bool {
	controller := metav1.GetControllerOf(pod)
	if controller == nil {
		return false
	}
	return controller.Kind == "DaemonSet"
}
