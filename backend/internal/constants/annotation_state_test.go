package constants

import "testing"

func TestAnnotationTransitions(t *testing.T) {
	tests := []struct {
		from, to string
		allowed  bool
	}{
		{AnnotationDraft, AnnotationSubmitted, true},
		{AnnotationSubmitted, AnnotationLocked, true},
		{AnnotationSubmitted, AnnotationReturned, true},
		{AnnotationReturned, AnnotationDraft, true},
		{AnnotationLocked, AnnotationCompared, true},
		{AnnotationCompared, AnnotationSuperseded, true},
		{AnnotationDraft, AnnotationCompared, false},
		{AnnotationSuperseded, AnnotationDraft, false},
	}
	for _, test := range tests {
		if actual := CanTransitionAnnotation(test.from, test.to); actual != test.allowed {
			t.Errorf("%s -> %s = %v, want %v", test.from, test.to, actual, test.allowed)
		}
	}
}

func TestAdjudicationTransitions(t *testing.T) {
	tests := []struct {
		from, to string
		allowed  bool
	}{
		{CaseOpen, CaseAssigned, true},
		{CaseAssigned, CaseAdjudicated, true},
		{CaseAdjudicated, CaseReviewed, true},
		{CaseReviewed, CaseAccepted, true},
		{CaseReviewed, CaseReopened, true},
		{CaseReopened, CaseAssigned, true},
		{CasePendingRecompute, CaseSuperseded, true},
		{CaseOpen, CaseAccepted, false},
		{CaseAccepted, CaseReopened, false},
		{CasePendingRecompute, CaseAssigned, false},
		{CaseSuperseded, CaseAssigned, false},
	}
	for _, test := range tests {
		if actual := CanTransitionCase(test.from, test.to); actual != test.allowed {
			t.Errorf("%s -> %s = %v, want %v", test.from, test.to, actual, test.allowed)
		}
	}
}

func TestRecomputableAndTerminalCaseStates(t *testing.T) {
	recomputable := map[string]bool{}
	for _, state := range CaseStatesThatCanBeRecomputed() {
		recomputable[state] = true
	}
	for _, state := range []string{CaseOpen, CaseAssigned, CaseAdjudicated, CaseReviewed, CaseReopened} {
		if !recomputable[state] {
			t.Errorf("state %s must be eligible for pending recomputation", state)
		}
		if CaseIsTerminalHistory(state) {
			t.Errorf("active state %s must not be treated as terminal history", state)
		}
	}
	for _, state := range []string{CaseAccepted, CaseSuperseded} {
		if recomputable[state] {
			t.Errorf("terminal state %s must never be moved back to recomputation", state)
		}
		if !CaseIsTerminalHistory(state) {
			t.Errorf("state %s must be read-only history", state)
		}
	}
}

func TestDatasetAndSchemaTransitions(t *testing.T) {
	if !CanTransitionDataset(DatasetDraft, DatasetFrozen) || CanTransitionDataset(DatasetDraft, DatasetArchived) {
		t.Fatal("dataset transition rules are incorrect")
	}
	if !CanTransitionSchema(SchemaDraft, SchemaValidated) || !CanTransitionSchema(SchemaValidated, SchemaPublished) {
		t.Fatal("schema transition rules are incorrect")
	}
	if CanTransitionSchema(SchemaDraft, SchemaPublished) {
		t.Fatal("schema publication cannot skip validation")
	}
}
