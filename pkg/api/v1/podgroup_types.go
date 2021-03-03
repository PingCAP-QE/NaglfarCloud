/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PodGroupSpec defines the desired state of PodGroup
type PodGroupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// MinMember defines the minimal number of members/tasks to run the pod group;
	// if there's not enough resources to start all tasks, the scheduler
	// will not start anyone.
	MinMember int32 `json:"minMember,omitempty"`

	// Exclusive is a flag to decide should this pod group monopolize nodes
	// +optional
	Exclusive *bool `json:"exclusive,omitempty"`

	// SubGroups is a list of sub pod groups
	// +optional
	SubGroups map[string]PodGroupSpec `json:"subGroups,omitempty"`
}

// PodGroupStatus defines the observed state of PodGroup
type PodGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// NextSchedulingTime is the time to schedule this pod group
	// +optional
	NextSchedulingTime *metav1.Time `json:"nextchedulingTime,omitempty"`
}

// +kubebuilder:object:root=true

// PodGroup is the Schema for the podgroups API
// +kubebuilder:subresource:status
type PodGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodGroupSpec   `json:"spec,omitempty"`
	Status PodGroupStatus `json:"status,omitempty"`
}

// IsExclusive is a wrapper of Exclusive field,
// it returns false if Exclusive field is nil.
func (pg *PodGroupSpec) IsExclusive() bool {
	if pg.Exclusive == nil {
		return false
	}
	return *pg.Exclusive
}

// SchedulingTime is a wrapper of NextSchedulingTime field of status,
// it returns create time if NextSchedulingTime field is nil.
func (pg *PodGroup) SchedulingTime() time.Time {
	if pg.Status.NextSchedulingTime == nil {
		return pg.CreationTimestamp.Time
	}

	return pg.Status.NextSchedulingTime.Time
}

// +kubebuilder:object:root=true

// PodGroupList contains a list of PodGroup
type PodGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodGroup{}, &PodGroupList{})
}
