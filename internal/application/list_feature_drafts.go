package application

import "fmt"

// ListFeatureDrafts lists the project’s saved feature documents.
type ListFeatureDrafts struct{ Store FeatureDraftStore }

// Execute delegates to the feature draft store.
func (u ListFeatureDrafts) Execute() ([]FeatureSummary, error) {
	result, err := u.Store.List()
	if err != nil {
		return result, fmt.Errorf("read feature: %w", err)
	}
	return result, nil
}
