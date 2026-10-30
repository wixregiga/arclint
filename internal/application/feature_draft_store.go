package application

import "errors"

// Feature transport and storage refusals.
var (
	ErrFeatureInput    = errors.New("invalid feature input")
	ErrFeatureMissing  = errors.New("feature not found")
	ErrFeatureConflict = errors.New("feature revision conflict")
)

// FeatureTerm is the recorded entry an author chose for one occurrence.
type FeatureTerm struct {
	Concept string `json:"concept"`
	Context string `json:"context"`
	Owner   string `json:"owner"`
	Name    string `json:"name"`
}

// FeatureReference retains an occurrence, its original wording, and the chosen reading.
type FeatureReference struct {
	ID      string       `json:"id"`
	From    int          `json:"from"`
	To      int          `json:"to"`
	Quote   string       `json:"quote"`
	Reading string       `json:"reading"`
	Term    *FeatureTerm `json:"term,omitempty"`
}

// FeatureDraft is a persisted application document, not a domain aggregate.
type FeatureDraft struct {
	ID         string             `json:"id"`
	Revision   string             `json:"revision"`
	Title      string             `json:"title"`
	Body       string             `json:"body"`
	References []FeatureReference `json:"references"`
}

// FeatureSummary identifies one saved document in the feature list.
type FeatureSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Revision string `json:"revision"`
}

// FeatureDraftStore persists documents and refuses saves or deletes of a stale revision.
type FeatureDraftStore interface {
	List() ([]FeatureSummary, error)
	Get(string) (FeatureDraft, error)
	Save(FeatureDraft) (FeatureDraft, error)
	Delete(string, string) error
}
