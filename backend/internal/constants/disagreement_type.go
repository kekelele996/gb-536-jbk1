package constants

const (
	DisagreementBoundary = "boundary"
	DisagreementLabel    = "label"
	DisagreementOmission = "omission"
	DisagreementOverlap  = "overlap"
)

func ValidDisagreementType(value string) bool {
	switch value {
	case DisagreementBoundary, DisagreementLabel, DisagreementOmission, DisagreementOverlap:
		return true
	default:
		return false
	}
}
