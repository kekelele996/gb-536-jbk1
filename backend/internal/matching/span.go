package matching

import (
	"fmt"
	"sort"
	"strings"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
)

func CompareAnnotations(left, right []dto.AnnotationLabel) ([]dto.DiffEvidence, []dto.ConfusionCell, string, string) {
	evidence := make([]dto.DiffEvidence, 0)
	confusionCounts := map[string]int{}
	leftClass, rightClass := classificationMap(left), classificationMap(right)
	classKeys := unionKeys(leftClass, rightClass)
	for _, unitKey := range classKeys {
		leftLabel, leftOK := leftClass[unitKey]
		rightLabel, rightOK := rightClass[unitKey]
		confusionCounts[confusionKey(valueOrMissing(leftLabel, leftOK), valueOrMissing(rightLabel, rightOK))]++
		switch {
		case !leftOK || !rightOK:
			evidence = append(evidence, dto.DiffEvidence{
				Type: constants.DisagreementOmission, UnitKey: unitKey,
				LeftLabel: valueOrMissing(leftLabel, leftOK), RightLabel: valueOrMissing(rightLabel, rightOK),
				Evidence: "one annotator omitted the classification unit",
			})
		case leftLabel != rightLabel:
			evidence = append(evidence, dto.DiffEvidence{
				Type: constants.DisagreementLabel, UnitKey: unitKey, LeftLabel: leftLabel, RightLabel: rightLabel,
				Evidence: fmt.Sprintf("classification label differs: %s versus %s", leftLabel, rightLabel),
			})
		}
	}

	leftSpans, rightSpans := spanLabels(left), spanLabels(right)
	usedRight := make([]bool, len(rightSpans))
	for _, leftSpan := range leftSpans {
		bestIndex, bestOverlap := -1, 0
		for index, rightSpan := range rightSpans {
			if usedRight[index] || leftSpan.UnitKey != rightSpan.UnitKey {
				continue
			}
			overlap := overlapLength(leftSpan, rightSpan)
			if exactBoundary(leftSpan, rightSpan) {
				bestIndex, bestOverlap = index, max(overlap, 1)
				break
			}
			if overlap > bestOverlap {
				bestIndex, bestOverlap = index, overlap
			}
		}
		if bestIndex < 0 || bestOverlap == 0 {
			confusionCounts[confusionKey(leftSpan.Label, "∅")]++
			evidence = append(evidence, omissionEvidence(leftSpan, true))
			continue
		}
		rightSpan := rightSpans[bestIndex]
		usedRight[bestIndex] = true
		confusionCounts[confusionKey(leftSpan.Label, rightSpan.Label)]++
		switch {
		case exactBoundary(leftSpan, rightSpan) && leftSpan.Label == rightSpan.Label:
			continue
		case exactBoundary(leftSpan, rightSpan):
			evidence = append(evidence, spanEvidence(constants.DisagreementLabel, leftSpan, rightSpan, bestOverlap,
				"the same character interval received different labels"))
		case leftSpan.Label == rightSpan.Label:
			evidence = append(evidence, spanEvidence(constants.DisagreementBoundary, leftSpan, rightSpan, bestOverlap,
				"matching labels use different character boundaries"))
		default:
			evidence = append(evidence, spanEvidence(constants.DisagreementOverlap, leftSpan, rightSpan, bestOverlap,
				"overlapping character intervals use different labels"))
		}
	}
	for index, rightSpan := range rightSpans {
		if usedRight[index] {
			continue
		}
		confusionCounts[confusionKey("∅", rightSpan.Label)]++
		evidence = append(evidence, omissionEvidence(rightSpan, false))
	}
	sort.SliceStable(evidence, func(left, right int) bool {
		if evidence[left].UnitKey == evidence[right].UnitKey {
			return evidence[left].LeftStart < evidence[right].LeftStart
		}
		return evidence[left].UnitKey < evidence[right].UnitKey
	})
	confusion := make([]dto.ConfusionCell, 0, len(confusionCounts))
	for key, count := range confusionCounts {
		parts := strings.SplitN(key, "\x00", 2)
		confusion = append(confusion, dto.ConfusionCell{LeftLabel: parts[0], RightLabel: parts[1], Count: count})
	}
	sort.Slice(confusion, func(left, right int) bool {
		if confusion[left].LeftLabel == confusion[right].LeftLabel {
			return confusion[left].RightLabel < confusion[right].RightLabel
		}
		return confusion[left].LeftLabel < confusion[right].LeftLabel
	})
	disagreementType, labelPair := dominantDisagreement(evidence)
	return evidence, confusion, disagreementType, labelPair
}

func ClusterKey(schemaCode, disagreementType, labelPair string) string {
	if disagreementType == "" {
		disagreementType = constants.DisagreementLabel
	}
	if labelPair == "" {
		labelPair = "agreement"
	}
	return strings.ToLower(fmt.Sprintf("%s:%s:%s", schemaCode, disagreementType, labelPair))
}

func classificationMap(labels []dto.AnnotationLabel) map[string]string {
	result := map[string]string{}
	for _, label := range labels {
		if !label.IsSpan() {
			result[label.UnitKey] = label.Label
		}
	}
	return result
}

func spanLabels(labels []dto.AnnotationLabel) []dto.AnnotationLabel {
	result := make([]dto.AnnotationLabel, 0)
	for _, label := range labels {
		if label.IsSpan() {
			result = append(result, label)
		}
	}
	return result
}

func unionKeys(left, right map[string]string) []string {
	unique := map[string]bool{}
	for key := range left {
		unique[key] = true
	}
	for key := range right {
		unique[key] = true
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func overlapLength(left, right dto.AnnotationLabel) int {
	start := max(left.Start, right.Start)
	end := min(left.End, right.End)
	return max(0, end-start)
}

func exactBoundary(left, right dto.AnnotationLabel) bool {
	return left.Start == right.Start && left.End == right.End
}

func spanEvidence(kind string, left, right dto.AnnotationLabel, overlap int, reason string) dto.DiffEvidence {
	return dto.DiffEvidence{
		Type: kind, UnitKey: left.UnitKey, LeftLabel: left.Label, RightLabel: right.Label,
		LeftStart: left.Start, LeftEnd: left.End, RightStart: right.Start, RightEnd: right.End,
		Overlap: overlap, Evidence: reason,
	}
}

func omissionEvidence(label dto.AnnotationLabel, missingRight bool) dto.DiffEvidence {
	evidence := dto.DiffEvidence{
		Type: constants.DisagreementOmission, UnitKey: label.UnitKey,
		Evidence: "one annotator omitted a span found by the other",
	}
	if missingRight {
		evidence.LeftLabel, evidence.LeftStart, evidence.LeftEnd = label.Label, label.Start, label.End
	} else {
		evidence.RightLabel, evidence.RightStart, evidence.RightEnd = label.Label, label.Start, label.End
	}
	return evidence
}

func dominantDisagreement(evidence []dto.DiffEvidence) (string, string) {
	if len(evidence) == 0 {
		return "", ""
	}
	counts := map[string]int{}
	pairs := map[string]map[string]int{}
	for _, item := range evidence {
		counts[item.Type]++
		left, right := item.LeftLabel, item.RightLabel
		if left == "" {
			left = "∅"
		}
		if right == "" {
			right = "∅"
		}
		pair := left + "~" + right
		if pairs[item.Type] == nil {
			pairs[item.Type] = map[string]int{}
		}
		pairs[item.Type][pair]++
	}
	priority := []string{constants.DisagreementOverlap, constants.DisagreementOmission, constants.DisagreementLabel, constants.DisagreementBoundary}
	selected, highest := "", -1
	for _, kind := range priority {
		if counts[kind] > highest {
			selected, highest = kind, counts[kind]
		}
	}
	selectedPair, pairCount := "", -1
	for pair, count := range pairs[selected] {
		if count > pairCount || (count == pairCount && pair < selectedPair) {
			selectedPair, pairCount = pair, count
		}
	}
	return selected, selectedPair
}

func confusionKey(left, right string) string { return left + "\x00" + right }

func valueOrMissing(value string, exists bool) string {
	if !exists {
		return "∅"
	}
	return value
}
