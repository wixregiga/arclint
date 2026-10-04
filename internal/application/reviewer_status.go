package application

import "fmt"

// ReviewerInstallation reports files and release identity, never review approval.
type ReviewerInstallation struct {
	Project, Path, Name, Host, InstalledVersion, AvailableVersion string
	Installed, Intact                                             bool
	Problems                                                      []string
}

// ReviewerInspector reads the native agent and its ownership evidence.
type ReviewerInspector interface {
	ReviewerStatus() (ReviewerInstallation, error)
}

// ReviewerStatus reports installation independently of host runtime state.
type ReviewerStatus struct{ inspector ReviewerInspector }

// NewReviewerStatus requires a read-only installation inspector.
func NewReviewerStatus(inspector ReviewerInspector) (ReviewerStatus, error) {
	if inspector == nil {
		return ReviewerStatus{}, fmt.Errorf("reviewer status: missing inspector")
	}
	return ReviewerStatus{inspector: inspector}, nil
}

// Execute validates the host and returns the current installed evidence.
func (uc ReviewerStatus) Execute(host string) (ReviewerInstallation, error) {
	if host != codexHost {
		return ReviewerInstallation{}, fmt.Errorf("reviewer status: host %q is unsupported; use codex", host)
	}
	status, err := uc.inspector.ReviewerStatus()
	if err != nil {
		return ReviewerInstallation{}, fmt.Errorf("reviewer status: %w", err)
	}
	return status, nil
}
