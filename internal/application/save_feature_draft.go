package application

import "fmt"

// SaveFeatureDraft saves a document through the configured store.
type SaveFeatureDraft struct{ Store FeatureDraftStore }

// Execute delegates to the feature draft store.
func (u SaveFeatureDraft) Execute(draft FeatureDraft) (FeatureDraft, error) {
	result, err := u.Store.Save(draft)
	if err != nil {
		return result, fmt.Errorf("save feature: %w", err)
	}
	return result, nil
}
