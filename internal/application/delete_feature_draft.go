package application

import "fmt"

// DeleteFeatureDraft carries the feature draft operation or its typed data.
type DeleteFeatureDraft struct{ Store FeatureDraftStore }

// Execute delegates to the feature draft store.
func (u DeleteFeatureDraft) Execute(id, revision string) error {
	err := u.Store.Delete(id, revision)
	if err != nil {
		return fmt.Errorf("delete feature: %w", err)
	}
	return nil
}
