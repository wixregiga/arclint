package main

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/delivery/cli"
)

func TestResolvePresentation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		args       []string
		noColorEnv string
		tty        bool
		wantName   cli.RendererName
		wantArgs   []string
		wantErr    string
	}{
		{
			name:     "default human non-tty is plain",
			args:     []string{"check", "."},
			tty:      false,
			wantName: cli.RendererPlain,
			wantArgs: []string{"check", "."},
		},
		{
			name:     "default human tty selects lipgloss",
			args:     []string{"check"},
			tty:      true,
			wantName: cli.RendererLipgloss,
			wantArgs: []string{"check"},
		},
		{
			name:     "format human split form",
			args:     []string{"--format", "human", "rules", "list"},
			tty:      true,
			wantName: cli.RendererLipgloss,
			wantArgs: []string{"rules", "list"},
		},
		{
			name:     "format human equal form",
			args:     []string{"--format=human", "check"},
			tty:      false,
			wantName: cli.RendererPlain,
			wantArgs: []string{"check"},
		},
		{
			name:     "format json split wins over tty",
			args:     []string{"check", "--format", "json"},
			tty:      true,
			wantName: cli.RendererJSON,
			wantArgs: []string{"check"},
		},
		{
			name:     "format json equal wins over color",
			args:     []string{"--format=json", "domain", "list"},
			tty:      true,
			wantName: cli.RendererJSON,
			wantArgs: []string{"domain", "list"},
		},
		{
			name:     "no-color split disables lipgloss on tty",
			args:     []string{"--no-color", "check"},
			tty:      true,
			wantName: cli.RendererPlain,
			wantArgs: []string{"check"},
		},
		{
			name:     "no-color equal true disables lipgloss",
			args:     []string{"--no-color=true", "check"},
			tty:      true,
			wantName: cli.RendererPlain,
			wantArgs: []string{"check"},
		},
		{
			name:     "no-color equal false keeps lipgloss on tty",
			args:     []string{"--no-color=false", "check"},
			tty:      true,
			wantName: cli.RendererLipgloss,
			wantArgs: []string{"check"},
		},
		{
			name:    "no-color equal invalid is config error",
			args:    []string{"--no-color=banana", "check"},
			wantErr: `invalid value "banana" for --no-color: strconv.ParseBool: parsing "banana": invalid syntax`,
		},
		{
			name:       "NO_COLOR non-empty disables lipgloss",
			args:       []string{"check"},
			noColorEnv: "1",
			tty:        true,
			wantName:   cli.RendererPlain,
			wantArgs:   []string{"check"},
		},
		{
			name:       "json still wins with NO_COLOR and no-color",
			args:       []string{"--no-color", "--format=json", "check"},
			noColorEnv: "yes",
			tty:        true,
			wantName:   cli.RendererJSON,
			wantArgs:   []string{"check"},
		},
		{
			name:     "double dash stops peeling",
			args:     []string{"check", "--", "--format", "json"},
			tty:      true,
			wantName: cli.RendererLipgloss,
			wantArgs: []string{"check", "--", "--format", "json"},
		},
		{
			name:     "value boundary after double dash preserved",
			args:     []string{"--format=json", "run", "--", "--no-color"},
			tty:      false,
			wantName: cli.RendererJSON,
			wantArgs: []string{"run", "--", "--no-color"},
		},
		{
			name:     "strips mixed process flags among command args",
			args:     []string{"--rules", "r.yaml", "check", "--format", "human", "--no-color", "pkg"},
			tty:      true,
			wantName: cli.RendererPlain,
			wantArgs: []string{"--rules", "r.yaml", "check", "pkg"},
		},
		{
			name:    "missing format value is config error",
			args:    []string{"check", "--format"},
			wantErr: `--format requires a value`,
		},
		{
			name:    "empty equal format is config error",
			args:    []string{"--format=", "check"},
			wantErr: `unknown format "" (human, json)`,
		},
		{
			name:    "invalid format is config error",
			args:    []string{"--format", "yaml", "check"},
			wantErr: `unknown format "yaml" (human, json)`,
		},
		{
			name:     "completion leaves format for cobra",
			args:     []string{"__complete", "check", "--format", ""},
			tty:      true,
			wantName: cli.RendererPlain,
			wantArgs: []string{"__complete", "check", "--format", ""},
		},
		{
			name:     "completionNoDesc leaves no-color",
			args:     []string{"__completeNoDesc", "--no-color", "check", "--format=json"},
			tty:      true,
			wantName: cli.RendererPlain,
			wantArgs: []string{"__completeNoDesc", "--no-color", "check", "--format=json"},
		},
		{
			name:     "completion command is not peeled",
			args:     []string{"completion", "bash"},
			tty:      true,
			wantName: cli.RendererPlain,
			wantArgs: []string{"completion", "bash"},
		},
		{
			name:     "non-tty human is plain even without no-color",
			args:     []string{"--format=human", "patterns"},
			tty:      false,
			wantName: cli.RendererPlain,
			wantArgs: []string{"patterns"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotName, gotArgs, err := resolvePresentation(tc.args, tc.noColorEnv, tc.tty)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("error = nil, want %q", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotName != tc.wantName {
				t.Errorf("name = %q, want %q", gotName, tc.wantName)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Errorf("args = %#v, want %#v", gotArgs, tc.wantArgs)
			}
		})
	}
}

func TestFirstPositionalSkipsProcessFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--format", "json", "--no-color", "check", "."}, "check"},
		{[]string{"--rules", "r.yaml", "init"}, "init"},
		{[]string{"--format=human", "--rules=r.yaml", "domain", "list"}, "domain"},
		{[]string{"--", "check"}, "check"},
		{[]string{"--format", "json", "--"}, ""},
		{[]string{"__complete", "check", "--format", ""}, "__complete"},
	}
	for _, tc := range cases {
		if got := firstPositional(tc.args); got != tc.want {
			t.Errorf("firstPositional(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}
