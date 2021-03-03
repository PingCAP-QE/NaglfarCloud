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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
)

// subGroupSelector is a label selector to selector all pods in a sub podgroup
type subGroupSelector []string

func newSubGroupSelector(prefixGroups []string) labels.Selector {
	return subGroupSelector(prefixGroups)
}

// Matches returns true if this selector matches the given set of labels.
func (s subGroupSelector) Matches(labels labels.Labels) bool {
	if !labels.Has(PodGroupLabel) {
		return false
	}

	names := strings.Split(labels.Get(PodGroupLabel), subGroupSeparator)
	if len(names) < len(s) {
		return false
	}

	return joinNames(names[0:len(s)]) == joinNames(s)
}

// Empty returns true if this selector does not restrict the selection space.
func (s subGroupSelector) Empty() bool {
	return len(s) == 0
}

// String returns a human readable string that represents this selector.
func (s subGroupSelector) String() string {
	return fmt.Sprintf("A label selector to selector all pods in a podgroup(%s)", joinNames(s))
}

// Add adds requirements to the Selector
func (s subGroupSelector) Add(r ...labels.Requirement) labels.Selector {
	return s
}

// Requirements converts this interface into Requirements to expose
// more detailed selection information.
// If there are querying parameters, it will return converted requirements and selectable=true.
// If this selector doesn't want to select anything, it will return selectable=false.
func (s subGroupSelector) Requirements() (labels.Requirements, bool) {
	return nil, true
}

// Make a deep copy of the selector.
func (s subGroupSelector) DeepCopySelector() labels.Selector {
	return newSubGroupSelector(strings.Split(joinNames(s), subGroupSeparator))
}

// RequiresExactMatch allows a caller to introspect whether a given selector
// requires a single specific label to be set, and if so returns the value it
// requires.
func (s subGroupSelector) RequiresExactMatch(label string) (value string, found bool) {
	if label == PodGroupLabel {
		return joinNames(s), true
	}
	return "", false
}
