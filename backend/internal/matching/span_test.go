package matching

import (
	"testing"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
)

func TestCompareAnnotationsClassifiesEvidence(t *testing.T) {
	left := []dto.AnnotationLabel{
		{UnitKey: "class-a", Label: "RISK"},
		{UnitKey: "class-b", Label: "CLEAR"},
		{UnitKey: "text", Label: "PARTY", Start: 0, End: 8},
		{UnitKey: "text", Label: "OBLIGATION", Start: 20, End: 30},
		{UnitKey: "text", Label: "PARTY", Start: 40, End: 50},
	}
	right := []dto.AnnotationLabel{
		{UnitKey: "class-a", Label: "CLEAR"},
		{UnitKey: "text", Label: "PARTY", Start: 0, End: 9},
		{UnitKey: "text", Label: "PARTY", Start: 20, End: 30},
		{UnitKey: "text", Label: "OBLIGATION", Start: 45, End: 55},
	}
	evidence, confusion, _, _ := CompareAnnotations(left, right)
	counts := map[string]int{}
	for _, item := range evidence {
		counts[item.Type]++
	}
	if counts[constants.DisagreementLabel] < 2 {
		t.Fatalf("expected classification and exact-span label evidence: %v", counts)
	}
	if counts[constants.DisagreementBoundary] != 1 {
		t.Fatalf("expected one boundary disagreement: %v", counts)
	}
	if counts[constants.DisagreementOmission] != 1 {
		t.Fatalf("expected one classification omission: %v", counts)
	}
	if counts[constants.DisagreementOverlap] != 1 {
		t.Fatalf("expected one overlapping-label disagreement: %v", counts)
	}
	if len(confusion) == 0 {
		t.Fatal("confusion matrix must not be empty")
	}
}

func TestCompareAnnotationsExactAgreement(t *testing.T) {
	labels := []dto.AnnotationLabel{
		{UnitKey: "class", Label: "CLEAR"},
		{UnitKey: "text", Label: "PARTY", Start: 2, End: 8},
	}
	evidence, confusion, kind, pair := CompareAnnotations(labels, labels)
	if len(evidence) != 0 {
		t.Fatalf("exact agreement produced evidence: %+v", evidence)
	}
	if len(confusion) != 2 {
		t.Fatalf("confusion matrix size = %d, want 2", len(confusion))
	}
	if kind != "" || pair != "" {
		t.Fatalf("exact agreement summary = %q %q", kind, pair)
	}
}

func TestClusterKeyIsStable(t *testing.T) {
	actual := ClusterKey("LEGAL-ENTITY", constants.DisagreementBoundary, "PARTY~PARTY")
	if actual != "legal-entity:boundary:party~party" {
		t.Fatalf("cluster key = %q", actual)
	}
}
