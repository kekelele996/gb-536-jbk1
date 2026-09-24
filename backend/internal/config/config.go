package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"corpus-annotation-agreement-control/backend/internal/algorithm"
	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/dto"
	"corpus-annotation-agreement-control/backend/internal/matching"
	"corpus-annotation-agreement-control/backend/internal/model"
)

type Config struct {
	Port               string
	DBDriver           string
	DBDSN              string
	DBAutoMigrate      bool
	JWTSecret          string
	JWTTTL             time.Duration
	CORSOrigin         string
	LogLevel           string
	RateLimitPerMinute int
	AlgorithmVersion   string
}

func Load() (Config, error) {
	cfg := Config{
		Port:               envString("PORT", "8080"),
		DBDriver:           envString("DB_DRIVER", "postgres"),
		DBDSN:              envString("DB_DSN", "host=localhost user=corpus_agreement password=corpus_agreement_local_536 dbname=corpus_agreement port=57536 sslmode=disable TimeZone=UTC"),
		DBAutoMigrate:      envBool("DB_AUTO_MIGRATE", true),
		JWTSecret:          envString("JWT_SECRET", ""),
		JWTTTL:             time.Duration(envInt("JWT_TTL_MINUTES", 480)) * time.Minute,
		CORSOrigin:         envString("CORS_ORIGIN", "http://localhost:18536"),
		LogLevel:           envString("LOG_LEVEL", "info"),
		RateLimitPerMinute: envInt("RATE_LIMIT_PER_MINUTE", 300),
		AlgorithmVersion:   envString("ALGORITHM_VERSION", "agreement-nominal-span-v1.0"),
	}
	if len(cfg.JWTSecret) < 24 {
		return Config{}, errors.New("JWT_SECRET must contain at least 24 bytes")
	}
	if cfg.RateLimitPerMinute < 30 || cfg.RateLimitPerMinute > 10000 {
		return Config{}, errors.New("RATE_LIMIT_PER_MINUTE must be between 30 and 10000")
	}
	if cfg.JWTTTL < 5*time.Minute || cfg.JWTTTL > 24*time.Hour {
		return Config{}, errors.New("JWT_TTL_MINUTES must be between 5 and 1440")
	}
	if cfg.AlgorithmVersion == "" {
		return Config{}, errors.New("ALGORITHM_VERSION is required")
	}
	return cfg, nil
}

func OpenDatabase(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DBDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DBDSN)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.DBDriver)
	}
	gormLogger := logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
		SlowThreshold: 200 * time.Millisecond, LogLevel: logger.Warn,
		IgnoreRecordNotFoundError: true, Colorful: false,
	})
	db, err := gorm.Open(dialector, &gorm.Config{Logger: gormLogger})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if cfg.DBAutoMigrate {
		if err := db.AutoMigrate(
			&model.User{}, &model.CorpusDataset{}, &model.AnnotationSchema{},
			&model.AnnotationSet{}, &model.AdjudicationCase{}, &model.AuditEvent{},
		); err != nil {
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	if err := seed(db, cfg); err != nil {
		return nil, fmt.Errorf("seed database: %w", err)
	}
	return db, nil
}

func seed(db *gorm.DB, cfg Config) error {
	accounts := []struct {
		username string
		password string
		role     string
	}{
		{"admin", "Admin#536", constants.RoleAdmin},
		{"manager", "Data#536", constants.RoleDataManager},
		{"annotator_a", "Annotate#536", constants.RoleAnnotator},
		{"annotator_b", "Compare#536", constants.RoleAnnotator},
		{"adjudicator", "Decide#536", constants.RoleAdjudicator},
		{"reviewer", "Review#536", constants.RoleAdjudicator},
		{"auditor", "Audit#536", constants.RoleAuditor},
	}
	for _, account := range accounts {
		var count int64
		if err := db.Model(&model.User{}).Where("username = ?", account.username).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(account.password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.Create(&model.User{
			Username: account.username, PasswordHash: string(hash), Role: account.role, Active: true,
		}).Error; err != nil {
			return err
		}
	}
	var datasetCount int64
	if err := db.Model(&model.CorpusDataset{}).Count(&datasetCount).Error; err != nil || datasetCount > 0 {
		return err
	}
	users := map[string]model.User{}
	for _, username := range []string{"manager", "annotator_a", "annotator_b"} {
		var user model.User
		if err := db.Where("username = ?", username).First(&user).Error; err != nil {
			return err
		}
		users[username] = user
	}
	dataset := model.CorpusDataset{
		DatasetCode: "NLP-LEGAL-ZH", Name: "Chinese contract clause corpus",
		Language: "zh-CN", Domain: "contract-risk", DocumentCount: 12840,
		ContentMaskPolicy: "Names and identifiers are replaced with [MASK] before review.",
		DatasetState:      constants.DatasetFrozen, Version: 4, OwnerTeam: "NLP Quality Lab",
		CreatedBy: users["manager"].ID,
	}
	if err := db.Create(&dataset).Error; err != nil {
		return err
	}
	definitions := []dto.LabelDefinition{
		{Code: "RISK", DisplayName: "Risk clause", TaskType: "classification", Description: "The unit contains a material contractual risk."},
		{Code: "CLEAR", DisplayName: "No material risk", TaskType: "classification", Description: "The unit is explicit and contains no material risk."},
		{Code: "PARTY", DisplayName: "Contract party", TaskType: "span", Description: "A masked party reference."},
		{Code: "OBLIGATION", DisplayName: "Obligation", TaskType: "span", Description: "A span describing a required action."},
	}
	examples := []dto.MaskedExample{
		{ItemKey: "EXAMPLE-01", MaskedText: "[MASK] shall provide *** within ten days.", Labels: json.RawMessage(`[{"unit_key":"clause","label":"RISK"}]`)},
	}
	definitionsJSON, _ := json.Marshal(definitions)
	examplesJSON, _ := json.Marshal(examples)
	published := time.Now().UTC().Add(-72 * time.Hour)
	schema := model.AnnotationSchema{
		DatasetID: dataset.ID, SchemaCode: "LEGAL-ENTITY", Version: 2,
		LabelDefinitionsJSON: string(definitionsJSON),
		SpanPolicy:           "Use half-open Unicode code-point offsets; exclude surrounding punctuation.",
		OverlapPolicy:        "Nested PARTY and OBLIGATION spans are forbidden; adjacent spans are allowed.",
		ExamplesJSON:         string(examplesJSON), SchemaState: constants.SchemaPublished,
		CreatedBy: users["manager"].ID, PublishedAt: &published,
	}
	if err := db.Create(&schema).Error; err != nil {
		return err
	}
	leftLabels := []dto.AnnotationLabel{
		{UnitKey: "document_class", Label: "RISK"},
		{UnitKey: "clause_1", Label: "CLEAR"},
		{UnitKey: "masked_clause", Label: "PARTY", Start: 0, End: 8},
		{UnitKey: "masked_clause", Label: "OBLIGATION", Start: 18, End: 32},
	}
	rightLabels := []dto.AnnotationLabel{
		{UnitKey: "document_class", Label: "CLEAR"},
		{UnitKey: "clause_1", Label: "CLEAR"},
		{UnitKey: "masked_clause", Label: "PARTY", Start: 0, End: 9},
		{UnitKey: "masked_clause", Label: "OBLIGATION", Start: 20, End: 32},
	}
	leftJSON, _ := json.Marshal(leftLabels)
	rightJSON, _ := json.Marshal(rightLabels)
	submitted := time.Now().UTC().Add(-30 * time.Hour)
	annotations := []model.AnnotationSet{
		{
			DatasetID: dataset.ID, SchemaID: schema.ID, AnnotatorID: users["annotator_a"].ID,
			ItemKey: "DOC-7F2A", LabelsJSON: string(leftJSON), SourceChecksum: digest("DOC-7F2A-source-v1"),
			AnnotationState: constants.AnnotationCompared, SubmittedAt: &submitted,
			QualityNote: "Boundary follows the published half-open offset rule.",
		},
		{
			DatasetID: dataset.ID, SchemaID: schema.ID, AnnotatorID: users["annotator_b"].ID,
			ItemKey: "DOC-7F2A", LabelsJSON: string(rightJSON), SourceChecksum: digest("DOC-7F2A-source-v1-b"),
			AnnotationState: constants.AnnotationCompared, SubmittedAt: &submitted,
			QualityNote: "Potential leading honorific included in party boundary.",
		},
	}
	if err := db.Create(&annotations).Error; err != nil {
		return err
	}
	ratingSets := []algorithm.RatingSet{
		{Coder: "annotator_a", Ratings: ratingMap(leftLabels)},
		{Coder: "annotator_b", Ratings: ratingMap(rightLabels)},
	}
	agreement, err := algorithm.Compute("auto", ratingSets)
	if err != nil {
		return err
	}
	evidence, confusion, disagreementType, labelPair := matching.CompareAnnotations(leftLabels, rightLabels)
	ids := []uint{annotations[0].ID, annotations[1].ID}
	idsJSON, _ := json.Marshal(ids)
	confusionJSON, _ := json.Marshal(confusion)
	evidenceJSON, _ := json.Marshal(evidence)
	inputHash := digest(fmt.Sprintf("%v|%s|%s", ids, annotations[0].LabelsJSON, annotations[1].LabelsJSON))
	adjudication := model.AdjudicationCase{
		DatasetID: dataset.ID, ItemKey: "DOC-7F2A", AnnotationSetIDsJSON: string(idsJSON),
		AgreementMetric: agreement.Metric, AgreementScore: agreement.Score,
		ObservedAgreement: agreement.ObservedAgreement, ChanceAgreement: agreement.ChanceAgreement,
		SampleSize: agreement.SampleSize, CoderCount: agreement.CoderCount,
		MissingValueCount: agreement.MissingValueCount, Applicability: agreement.Applicability,
		DisagreementType: disagreementType, ConfusionSnapshotJSON: string(confusionJSON),
		EvidenceSnapshotJSON: string(evidenceJSON),
		ClusterKey:           matching.ClusterKey(schema.SchemaCode, disagreementType, labelPair),
		CaseState:            constants.CaseOpen, FinalLabelsJSON: "[]", Rationale: "",
		InputHash: inputHash, AlgorithmVersion: cfg.AlgorithmVersion,
		IdempotencyKey: "seed-agreement-doc-7f2a", CreatedBy: users["manager"].ID,
	}
	return db.Create(&adjudication).Error
}

func ratingMap(labels []dto.AnnotationLabel) map[string]string {
	result := map[string]string{}
	for _, label := range labels {
		key := label.UnitKey
		if label.IsSpan() {
			key = fmt.Sprintf("%s:%d-%d", label.UnitKey, label.Start, label.End)
		}
		result[key] = label.Label
	}
	return result
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
