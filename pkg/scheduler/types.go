package scheduler

import (
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

const (
	// PodGroupLabel is the default label of naglfar scheduler
	PodGroupLabel           = "naglfar/podgroup"
	subGroupSeparator       = "."
	exclusiveNodeAnnotation = "naglfar/exclusive"

	// for elastic exclusive node preemption
	estimationDurationAnnotation    = "estimation-duration"
	tolerateEstimationDurationValue = "short-term"
)

var (
	_ labels.Selector = PodGroupNameSlice{}
)

// PodGroupNameSlice is type alias of []string
type PodGroupNameSlice []string

// ParsePodGroupNameSliceFromStr parses podGroupNameSlice from raw string
func ParsePodGroupNameSliceFromStr(pgName string) PodGroupNameSlice {
	if pgName == "" {
		return nil
	}
	return strings.Split(pgName, subGroupSeparator)
}

func (p PodGroupNameSlice) PodGroupName() string {
	return strings.Join(p, subGroupSeparator)
}

func (p PodGroupNameSlice) IsEmpty() bool {
	return len(p) == 0
}

// IsUpperPodGroupOf checks whether thiis belongs to ancestor
func (p PodGroupNameSlice) IsBelongTo(ancestor PodGroupNameSlice) bool {
	if p.IsEmpty() || ancestor.IsEmpty() {
		return false
	}
	if len(p) < len(ancestor) {
		return false
	}
	for i := range ancestor {
		if p[i] != ancestor[i] {
			return false
		}
	}
	return true
}

// Matches returns true if this selector matches the given set of labels.
func (p PodGroupNameSlice) Matches(labels labels.Labels) bool {
	if !labels.Has(PodGroupLabel) {
		return false
	}

	pgns := ParsePodGroupNameSliceFromStr(labels.Get(PodGroupLabel))
	return pgns.IsBelongTo(p)
}

// Empty returns true if this selector does not restrict the selection space.
func (p PodGroupNameSlice) Empty() bool {
	return len(p) == 0
}

// String returns a human readable string that represents this selector.
func (p PodGroupNameSlice) String() string {
	return p.PodGroupName() + "/**"
}

// Add adds requirements to the Selector
func (p PodGroupNameSlice) Add(r ...labels.Requirement) labels.Selector {
	return p
}

// Requirements converts this interface into Requirements to expose
// more detailed selection information.
// If there are querying parameters, it will return converted requirements and selectable=true.
// If this selector doesn't want to select anything, it will return selectable=false.
func (p PodGroupNameSlice) Requirements() (labels.Requirements, bool) {
	return nil, true
}

// Make a deep copy of the selector.
func (p PodGroupNameSlice) DeepCopySelector() labels.Selector {
	return ParsePodGroupNameSliceFromStr(p.PodGroupName())
}

// RequiresExactMatch allows a caller to introspect whether a given selector
// requires a single specific label to be set, and if so returns the value it
// requires.
func (p PodGroupNameSlice) RequiresExactMatch(label string) (value string, found bool) {
	if label == PodGroupLabel {
		return p.PodGroupName(), true
	}
	return "", false
}

// NoOp ...
type NoOp struct{}

func (n NoOp) Clone() framework.StateData {
	return n
}
