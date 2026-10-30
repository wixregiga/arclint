package application

import "fmt"

// SaveFeatureDraft saves a document through the configured store.
type SaveFeatureDraft struct{ Store FeatureDraftStore }

// Execute checks the document shape, then delegates to the feature draft store.
func (u SaveFeatureDraft) Execute(draft FeatureDraft) (FeatureDraft, error) {
	if err := CheckFeatureDraft(draft); err != nil {
		return draft, fmt.Errorf("save feature: %w", err)
	}
	result, err := u.Store.Save(draft)
	if err != nil {
		return result, fmt.Errorf("save feature: %w", err)
	}
	return result, nil
}
