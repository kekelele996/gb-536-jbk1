package algorithm

import (
	"math"
	"testing"
)

func TestCohenKappaKnownFixture(t *testing.T) {
	left := RatingSet{Coder: "left", Ratings: map[string]string{
		"u1": "A", "u2": "A", "u3": "B", "u4": "B", "u5": "C",
	}}
	right := RatingSet{Coder: "right", Ratings: map[string]string{
		"u1": "A", "u2": "B", "u3": "B", "u4": "B", "u5": "C",
	}}
	result, err := CohenKappa(left, right)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, result.ObservedAgreement, 0.8)
	assertClose(t, result.ChanceAgreement, 0.36)
	assertClose(t, result.Score, 0.6875)
	if result.SampleSize != 5 || result.CoderCount != 2 {
		t.Fatalf("unexpected sample metadata: %+v", result)
	}
}

func TestKrippendorffAlphaSupportsMissingRatings(t *testing.T) {
	sets := []RatingSet{
		{Coder: "a", Ratings: map[string]string{"u1": "X", "u2": "Y", "u3": "X"}},
		{Coder: "b", Ratings: map[string]string{"u1": "X", "u2": "Y", "u3": "X"}},
		{Coder: "c", Ratings: map[string]string{"u1": "X", "u3": "X"}},
	}
	result, err := KrippendorffAlpha(sets)
	if err != nil {
		t.Fatal(err)
	}
	assertClose(t, result.Score, 1)
	assertClose(t, result.ObservedAgreement, 1)
	if result.MissingValueCount != 1 || result.SampleSize != 3 {
		t.Fatalf("missing/sample metadata = %+v", result)
	}
	if ChooseMetric(sets) != "krippendorff_alpha" {
		t.Fatal("multiple coders must select Krippendorff's Alpha")
	}
}

func TestAgreementMetricBoundaryConditions(t *testing.T) {
	if _, err := CohenKappa(RatingSet{Ratings: map[string]string{"u1": "A"}}, RatingSet{Ratings: map[string]string{}}); err == nil {
		t.Fatal("expected no shared ratings error")
	}
	if _, err := KrippendorffAlpha([]RatingSet{{Ratings: map[string]string{"u1": "A"}}}); err == nil {
		t.Fatal("expected coder count error")
	}
	twoWithMissing := []RatingSet{
		{Ratings: map[string]string{"u1": "A", "u2": "B"}},
		{Ratings: map[string]string{"u1": "A"}},
	}
	if ChooseMetric(twoWithMissing) != "krippendorff_alpha" {
		t.Fatal("missing ratings must select Krippendorff's Alpha")
	}
}

func assertClose(t *testing.T, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 1e-6 {
		t.Fatalf("value = %.8f, want %.8f", actual, expected)
	}
}
