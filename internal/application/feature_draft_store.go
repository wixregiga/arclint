package application

import "errors"

// Feature transport and storage refusals.
var (
	ErrFeatureInput    = errors.New("invalid feature input")
	ErrFeatureMissing  = errors.New("feature not found")
	ErrFeatureConflict = errors.New("feature revision conflict")
)

// FeatureTerm carries the feature draft operation or its typed data.
type FeatureTerm struct {
	Concept string `json:"concept"`
	Context string `json:"context"`
	Owner   string `json:"owner"`
	Name    string `json:"name"`
}

// FeatureReference carries the feature draft operation or its typed data.
type FeatureReference struct {
	ID      string       `json:"id"`
	From    int          `json:"from"`
	To      int          `json:"to"`
	Quote   string       `json:"quote"`
	Reading string       `json:"reading"`
	Term    *FeatureTerm `json:"term,omitempty"`
}

// FeatureDraft is a persisted application document, not a domain aggregate.
// FeatureDraft carries the feature draft operation or its typed data.
type FeatureDraft struct {
	ID         string             `json:"id"`
	Revision   string             `json:"revision"`
	Title      string             `json:"title"`
	Body       string             `json:"body"`
	References []FeatureReference `json:"references"`
}

// FeatureSummary carries the feature draft operation or its typed data.
type FeatureSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Revision string `json:"revision"`
}

// FeatureDraftStore carries the feature draft operation or its typed data.
type FeatureDraftStore interface {
	List() ([]FeatureSummary, error)
	Get(string) (FeatureDraft, error)
	Save(FeatureDraft) (FeatureDraft, error)
	Delete(string, string) error
}
