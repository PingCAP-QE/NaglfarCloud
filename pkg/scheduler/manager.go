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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	informerv1 "k8s.io/client-go/informers/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// PodGroupLabel is the default label of naglfar scheduler
const PodGroupLabel = "podgroup.naglfar"

type PodGroupManager struct {
	// snapshotSharedLister is pod shared list
	snapshotSharedLister framework.SharedLister

	// scheduleTimeout is the default time when group scheduling.
	// If podgroup's ScheduleTimeoutSeconds set, that would be used.
	scheduleTimeout time.Duration

	// podLister is pod lister
	podLister listerv1.PodLister
}

func NewPodGroupManager(snapshotSharedLister framework.SharedLister, scheduleTimeout time.Duration, podInformer informerv1.PodInformer) *PodGroupManager {
	return &PodGroupManager{
		snapshotSharedLister: snapshotSharedLister,
		scheduleTimeout:      scheduleTimeout,
		podLister:            podInformer.Lister(),
	}
}

// PreFilter filters out a pod if it
func (mgr *PodGroupManager) PreFilter(ctx context.Context, pod *corev1.Pod) error {
	klog.V(5).Infof("Pre-filter %v", pod.Name)
	return nil
}

// Permit permits a pod to run
func (mgr *PodGroupManager) Permit(ctx context.Context, pod *corev1.Pod, nodeName string) (bool, error) {
	pgName := PodGroup(pod)

	if pgName == "" {
		// no group
		return true, nil
	}

	assigned := mgr.CalculateAssignedPods(pod.Namespace, pgName)
	// The number of pods that have been assigned nodes is calculated from the snapshot.
	// The current pod in not included in the snapshot during the current scheduling cycle.
	if assigned+1 < mgr.CalculateAllPods(pod.Namespace, pgName) {
		return false, fmt.Errorf("waiting")
	}
	return true, nil
}

// PodGroup returns the name of PodGroup that a Pod belongs to.
func PodGroup(pod *corev1.Pod) string {
	return pod.Labels[PodGroupLabel]
}

func (mgr *PodGroupManager) CalculateAllPods(namespace, pgName string) int {
	pods, err := mgr.podLister.Pods(namespace).List(labels.SelectorFromSet(labels.Set(map[string]string{PodGroupLabel: pgName})))
	if err != nil {
		klog.Errorf("Cannot list pods from frameworkHandle: %v", err)
		return 0
	}

	return len(pods)
}

func (mgr *PodGroupManager) CalculateAssignedPods(namespace, pgName string) int {
	nodeInfos, err := mgr.snapshotSharedLister.NodeInfos().List()
	if err != nil {
		klog.Errorf("Cannot get nodeInfos from frameworkHandle: %v", err)
		return 0
	}
	var count int
	for _, nodeInfo := range nodeInfos {
		for _, podInfo := range nodeInfo.Pods {
			pod := podInfo.Pod
			if PodGroup(pod) == pgName && pod.Namespace == namespace && pod.Spec.NodeName != "" {
				count++
			}
		}
	}

	return count
}
