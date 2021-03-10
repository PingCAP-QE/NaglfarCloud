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

package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/PingCAP-QE/NaglfarCloud/pkg/scheduler"
)

var labelerLog = ctrl.Log.WithName("labeler")

const PodGroupAnnotaion = scheduler.PodGroupLabel

// PodLabeler is a mutable webhook for pod, used to add the pod group label for pod by it annotations.
// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kb.io
type PodLabeler struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (a *PodLabeler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	labelerLog.Info("handle pod %v", pod)

	// mutate the fields in pod
	if pgAnnotation, ok := pod.Annotations[PodGroupAnnotaion]; ok {
		if _, ok := pod.Labels[scheduler.PodGroupLabel]; !ok {
			// add default label
			labelerLog.Info("set %s=%s for pod %s/%s", scheduler.PodGroupLabel, pgAnnotation, pod.Namespace, pod.Name)
			pod.Labels[scheduler.PodGroupLabel] = pgAnnotation
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (a *PodLabeler) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
