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
	}
	return allowed[from][to]
}

// Compared and locked results leave the regular lifecycle only through the
// audited replacement flow, which jumps directly to "superseded" while a new
// draft inherits the annotator and item.
func CanReplaceAnnotation(from string) bool {
	return from == AnnotationLocked || from == AnnotationCompared
}

func CanTransitionCase(from, to string) bool {
	allowed := map[string]map[string]bool{
		CaseOpen:        {CaseAssigned: true},
		CaseReopened:    {CaseAssigned: true},
		CaseAssigned:    {CaseAdjudicated: true},
		CaseAdjudicated: {CaseReviewed: true},
		CaseReviewed:    {CaseAccepted: true, CaseReopened: true},
	}
	return allowed[from][to]
}

// ActiveCaseStates are the adjudication states that may still be worked on.
// Accepted history is read-only and pending_recompute waits for a fresh
// comparison against the replacement annotation version.
func CaseIsActive(state string) bool {
	switch state {
	case CaseOpen, CaseAssigned, CaseAdjudicated, CaseReviewed, CaseReopened:
		return true
	default:
		return false
	}
}
