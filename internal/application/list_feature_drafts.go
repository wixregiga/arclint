package application

import "fmt"

// ListFeatureDrafts carries the feature draft operation or its typed data.
type ListFeatureDrafts struct{ Store FeatureDraftStore }

// Execute delegates to the feature draft store.
func (u ListFeatureDrafts) Execute() ([]FeatureSummary, error) {
	result, err := u.Store.List()
	if err != nil {
		return result, fmt.Errorf("read feature: %w", err)
	}
	return result, nil
}
