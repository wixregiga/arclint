package application

import "fmt"

// SaveFeatureDraft carries the feature draft operation or its typed data.
type SaveFeatureDraft struct{ Store FeatureDraftStore }

// Execute delegates to the feature draft store.
func (u SaveFeatureDraft) Execute(draft FeatureDraft) (FeatureDraft, error) {
	result, err := u.Store.Save(draft)
	if err != nil {
		return result, fmt.Errorf("save feature: %w", err)
	}
	return result, nil
}
