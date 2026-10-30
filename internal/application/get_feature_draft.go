package application

import "fmt"

// GetFeatureDraft carries the feature draft operation or its typed data.
type GetFeatureDraft struct{ Store FeatureDraftStore }

// Execute delegates to the feature draft store.
func (u GetFeatureDraft) Execute(id string) (FeatureDraft, error) {
	result, err := u.Store.Get(id)
	if err != nil {
		return result, fmt.Errorf("read feature: %w", err)
	}
	return result, nil
}
