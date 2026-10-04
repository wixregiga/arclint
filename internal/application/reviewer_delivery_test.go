package application

import (
	"errors"
	"reflect"
	"testing"
)

type reviewerDeliveryProbe struct {
	installs, inspections int
	failure               error
}

func (p *reviewerDeliveryProbe) InstallReviewer() ([]string, error) {
	p.installs++
	return []string{"owned reviewer configuration"}, p.failure
}

func (p *reviewerDeliveryProbe) ReviewerStatus() (ReviewerInstallation, error) {
	p.inspections++
	return ReviewerInstallation{Host: "codex", Installed: true}, p.failure
}

func TestReviewerUnsupportedHostNeverInvokesDelivery(t *testing.T) {
	for _, host := range []string{"", "omp", "Codex", " codex"} {
		t.Run(host, func(t *testing.T) {
			probe := &reviewerDeliveryProbe{}
			install, err := NewInstallReviewer(probe)
			if err != nil {
				t.Fatal(err)
			}
			status, err := NewReviewerStatus(probe)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := install.Execute(host); err == nil {
				t.Fatal("accepted unsupported installation host")
			}
			if _, err := status.Execute(host); err == nil {
				t.Fatal("accepted unsupported inspection host")
			}
			if probe.installs != 0 || probe.inspections != 0 {
				t.Fatalf("delivery invoked: %+v", probe)
			}
		})
	}
}

func TestReviewerDeliveryReturnsAdapterEvidenceAndFailures(t *testing.T) {
	probe := &reviewerDeliveryProbe{}
	install, _ := NewInstallReviewer(probe)
	status, _ := NewReviewerStatus(probe)
	paths, err := install.Execute("codex")
	if err != nil || !reflect.DeepEqual(paths, []string{"owned reviewer configuration"}) {
		t.Fatalf("paths %v: %v", paths, err)
	}
	evidence, err := status.Execute("codex")
	if err != nil || !evidence.Installed || evidence.Host != "codex" {
		t.Fatalf("evidence %+v: %v", evidence, err)
	}
	failure := errors.New("delivery unavailable")
	probe.failure = failure
	if _, err := install.Execute("codex"); !errors.Is(err, failure) {
		t.Fatalf("lost install failure: %v", err)
	}
	if _, err := status.Execute("codex"); !errors.Is(err, failure) {
		t.Fatalf("lost status failure: %v", err)
	}
}

func TestReviewerDeliveryRequiresPorts(t *testing.T) {
	if _, err := NewInstallReviewer(nil); err == nil {
		t.Fatal("accepted missing installer")
	}
	if _, err := NewReviewerStatus(nil); err == nil {
		t.Fatal("accepted missing inspector")
	}
}
