package service

import (
	"strings"
	"testing"

	"corpus-annotation-agreement-control/backend/internal/model"
)

func TestEncodeSummaryRecursivelyRedactsContent(t *testing.T) {
	encoded := encodeSummary(map[string]any{
		"dataset_id": 17,
		"labels":     []any{map[string]any{"label": "SECRET_LABEL"}},
		"nested": map[string]any{
			"raw_text":      "private source sentence",
			"examples":      []any{map[string]any{"masked_text": "still sensitive"}},
			"access-token":  "bearer-secret-value",
			"password_hash": "hashed-secret-value",
		},
		"safe": []any{map[string]any{"source_checksum": "abc123"}},
	})
	for _, forbidden := range []string{"SECRET_LABEL", "private source sentence", "still sensitive", "bearer-secret-value", "hashed-secret-value"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("audit summary leaked %q: %s", forbidden, encoded)
		}
	}
	decoded := decodeSummary(encoded)
	if decoded["labels"] != "[REDACTED]" {
		t.Fatalf("labels were not redacted: %#v", decoded)
	}
	nested, ok := decoded["nested"].(map[string]any)
	if !ok || nested["raw_text"] != "[REDACTED]" || nested["examples"] != "[REDACTED]" ||
		nested["access-token"] != "[REDACTED]" || nested["password_hash"] != "[REDACTED]" {
		t.Fatalf("nested content was not redacted: %#v", decoded)
	}
}

func TestAdjudicationResponsePreservesAgreementMetadata(t *testing.T) {
	response := adjudicationResponse(model.AdjudicationCase{
		AnnotationSetIDsJSON: "[1,2,3]", ConfusionSnapshotJSON: "[]",
		EvidenceSnapshotJSON: "[]", FinalLabelsJSON: "[]",
		CoderCount: 3, MissingValueCount: 2,
	})
	if response.Agreement.CoderCount != 3 || response.Agreement.MissingValueCount != 2 {
		t.Fatalf("agreement metadata was not preserved: %+v", response.Agreement)
	}
}
