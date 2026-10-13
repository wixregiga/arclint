package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
)

type agentsContextReader struct {
	block string
	found bool
	err   error
}

func (r agentsContextReader) GeneratedBlock() (string, bool, error) {
	return r.block, r.found, r.err
}

func TestInspectAgentsContextUsesThePublisherWithoutInstalling(t *testing.T) {
	publisher := &fakePublisher{}
	publish, err := application.NewPublishAgentsContext(fakeRepository{agentsFixture(t)}, recordedKnowledge(t), fakeExtensionInventory{}, publisher)
	if err != nil {
		t.Fatal(err)
	}
	block, err := publish.Render()
	if err != nil {
		t.Fatal(err)
	}
	readFailure := errors.New("document unreadable")
	for _, tc := range []struct {
		name   string
		reader agentsContextReader
		want   string
		err    error
	}{
		{name: "absent"},
		{name: "current", reader: agentsContextReader{block: strings.TrimSuffix(block, "\n"), found: true}},
		{name: "current with CRLF line endings", reader: agentsContextReader{block: strings.ReplaceAll(strings.TrimSuffix(block, "\n"), "\n", "\r\n"), found: true}},
		{name: "stale", reader: agentsContextReader{block: "old", found: true}, want: application.AgentsContextStaleMessage},
		{name: "read failure", reader: agentsContextReader{err: readFailure}, err: readFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspect, err := application.NewInspectAgentsContext(publish, tc.reader)
			if err != nil {
				t.Fatal(err)
			}
			got, err := inspect.Execute()
			if got != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("Execute = (%q, %v), want (%q, %v)", got, err, tc.want, tc.err)
			}
			if publisher.installed != "" {
				t.Fatal("inspection installed a block")
			}
		})
	}
}

func TestInspectAgentsContextWithoutABlockDoesNotRender(t *testing.T) {
	inspect, err := application.NewInspectAgentsContext(application.PublishAgentsContext{}, agentsContextReader{})
	if err != nil {
		t.Fatal(err)
	}
	if advice, err := inspect.Execute(); err != nil || advice != "" {
		t.Fatalf("without a block: %q %v", advice, err)
	}
	if _, err := application.NewInspectAgentsContext(application.PublishAgentsContext{}, nil); err == nil {
		t.Fatal("missing reader accepted")
	}
}
