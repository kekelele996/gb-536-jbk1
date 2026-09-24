package constants

const (
	RoleAdmin       = "admin"
	RoleDataManager = "data_manager"
	RoleAnnotator   = "annotator"
	RoleAdjudicator = "adjudicator"
	RoleAuditor     = "auditor"

	DatasetDraft    = "draft"
	DatasetFrozen   = "frozen"
	DatasetArchived = "archived"

	SchemaDraft      = "draft"
	SchemaValidated  = "validated"
	SchemaPublished  = "published"
	SchemaDeprecated = "deprecated"

	AnnotationDraft      = "draft"
	AnnotationSubmitted  = "submitted"
	AnnotationReturned   = "returned"
	AnnotationLocked     = "locked"
	AnnotationCompared   = "compared"
	AnnotationSuperseded = "superseded"

	CaseOpen             = "open"
	CaseAssigned         = "assigned"
	CaseAdjudicated      = "adjudicated"
	CaseReviewed         = "reviewed"
	CaseAccepted         = "accepted"
	CaseReopened         = "reopened"
	CasePendingRecompute = "pending_recompute"
	CaseSuperseded       = "superseded"
)

func ValidRole(value string) bool {
	switch value {
	case RoleAdmin, RoleDataManager, RoleAnnotator, RoleAdjudicator, RoleAuditor:
		return true
	default:
		return false
	}
}

func ValidAnnotationState(value string) bool {
	switch value {
	case AnnotationDraft, AnnotationSubmitted, AnnotationReturned, AnnotationLocked, AnnotationCompared, AnnotationSuperseded:
		return true
	default:
		return false
	}
}

func CanTransitionDataset(from, to string) bool {
	return (from == DatasetDraft && to == DatasetFrozen) ||
		(from == DatasetFrozen && to == DatasetArchived)
}

func CanTransitionSchema(from, to string) bool {
	return (from == SchemaDraft && to == SchemaValidated) ||
		(from == SchemaValidated && to == SchemaPublished) ||
		(from == SchemaPublished && to == SchemaDeprecated)
}

func CanTransitionAnnotation(from, to string) bool {
	allowed := map[string]map[string]bool{
		AnnotationDraft:     {AnnotationSubmitted: true},
		AnnotationSubmitted: {AnnotationLocked: true, AnnotationReturned: true},
		AnnotationReturned:  {AnnotationDraft: true},
		AnnotationLocked:    {AnnotationCompared: true},
		AnnotationCompared:  {AnnotationSuperseded: true},
	}
	return allowed[from][to]
}

// CaseStatesThatCanBeRecomputed lists cases whose evidence may still drive the
// current decision. When an input annotation is replaced these cases must leave
// the active queue and wait for a fresh comparison. Accepted cases are locked
// history and are deliberately excluded.
func CaseStatesThatCanBeRecomputed() []string {
	return []string{CaseOpen, CaseAssigned, CaseAdjudicated, CaseReviewed, CaseReopened}
}

// CaseIsTerminalHistory reports states that can no longer drive a decision and
// exist only for inspection.
func CaseIsTerminalHistory(value string) bool {
	return value == CaseAccepted || value == CaseSuperseded
}

func CanTransitionCase(from, to string) bool {
	allowed := map[string]map[string]bool{
		CaseOpen:             {CaseAssigned: true},
		CaseReopened:         {CaseAssigned: true},
		CaseAssigned:         {CaseAdjudicated: true},
		CaseAdjudicated:      {CaseReviewed: true},
		CaseReviewed:         {CaseAccepted: true, CaseReopened: true},
		CasePendingRecompute: {CaseSuperseded: true},
	}
	return allowed[from][to]
}
