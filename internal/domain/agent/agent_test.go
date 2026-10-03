package agent_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/agent"
)

func TestAgentPreservesAuthoredDefinition(t *testing.T) {
	const name = "arclint-domain-reviewer"
	const description = " Explain domain review findings. "
	const instructions = "First inspect the domain.\n\nExplain evidence before suggesting a repair.\n"
	first, err := agent.New(name, description, instructions)
	if err != nil {
		t.Fatal(err)
	}
	if first.Name() != name || first.Description() != description || first.Instructions() != instructions {
		t.Fatalf("definition changed: %#v", first)
	}
	second, err := agent.New(name, description, instructions)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("equal definitions are not interchangeable")
	}
	changed, err := agent.New(name, description, instructions+"Disclose missing history.\n")
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("changed instructions did not produce another definition")
	}
}

func TestAgentRejectsUnusableDefinitions(t *testing.T) {
	for _, name := range []string{"", "../reviewer", "a/b", `a\b`, "-reviewer", "reviewer-", "two--words", "DomainReviewer", "reviewer.toml", "1-reviewer", "reviewer\n"} {
		t.Run("name="+name, func(t *testing.T) {
			if _, err := agent.New(name, "Review domain decisions", "Explain evidence"); err == nil {
				t.Fatalf("accepted unusable name %q", name)
			}
		})
	}
	for _, test := range []struct {
		name, description, instructions, want string
	}{
		{"empty description", "", "Explain evidence", "purpose description"},
		{"blank description", " \t\n", "Explain evidence", "purpose description"},
		{"empty instructions", "Review domain decisions", "", "instructions"},
		{"blank instructions", "Review domain decisions", " \t\n", "instructions"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := agent.New("reviewer", test.description, test.instructions); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %s error, got %v", test.want, err)
			}
		})
	}
	if err := (agent.Agent{}).Validate(); err == nil {
		t.Fatal("zero definition is usable")
	}
}

func TestAgentHostSelectsOnlyCodexDelivery(t *testing.T) {
	host, err := agent.NewHost("codex")
	if err != nil {
		t.Fatal(err)
	}
	if host.String() != "codex" || host.Validate() != nil {
		t.Fatalf("unexpected host: %#v", host)
	}
	for _, name := range []string{"", "omp", "claude", "Codex", " codex ", "codex\n"} {
		if _, err := agent.NewHost(name); err == nil {
			t.Errorf("accepted unsupported host %q", name)
		}
	}
	if err := (agent.Host{}).Validate(); err == nil {
		t.Fatal("zero host selects a delivery")
	}
}
