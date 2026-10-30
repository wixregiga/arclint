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

// CheckFeatureDraft checks the document shape without resolving its annotations.
// Collapsed and out-of-body ranges remain available as unresolved references.
func CheckFeatureDraft(d FeatureDraft) error {
	if d.References == nil {
		return ErrFeatureInput
	}
	ids := make(map[string]bool, len(d.References))
	for _, r := range d.References {
		if r.ID == "" || ids[r.ID] || r.From < 0 || r.To < r.From {
			return ErrFeatureInput
		}
		ids[r.ID] = true
		switch r.Reading {
		case "plain":
			if r.Term != nil {
				return ErrFeatureInput
			}
		case "names", "value", "rule":
			if r.Term == nil || r.Term.Concept == "" || r.Term.Name == "" {
				return ErrFeatureInput
			}
		default:
			return ErrFeatureInput
		}
	}
	return nil
}
