package hooks_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/infrastructure/agents/hooks"
)

func TestRecordingStateResolvesDomainLocation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		external bool
		file     bool
		missing  bool
	}{
		{"outside directory", true, false, false},
		{"outside missing file", true, false, true},
		{"outside file", true, true, false},
		{"dangling file", true, true, true},
		{"inside directory", false, false, false},
		{"inside file", false, true, false},
		{"inside missing file", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "actual")
			if tc.external {
				directory = filepath.Join(t.TempDir(), "actual")
			}
			if err := os.Mkdir(directory, 0o750); err != nil {
				t.Fatal(err)
			}
			const content = "version: 1\nproject: shop\n"
			target := filepath.Join(directory, "domain.yaml")
			if !tc.missing {
				if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path, linkTarget := "model/domain.yaml", directory
			link := filepath.Join(root, "model")
			if tc.file {
				path, linkTarget = "domain.yaml", target
				link = filepath.Join(root, path)
			}
			if err := os.Symlink(linkTarget, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			state := hooks.NewActivityLog(root, path).RecordingState()
			if tc.external {
				if !strings.HasPrefix(state, "unreadable: DomainRecording inside-the-repository:") {
					t.Fatalf("external file state = %s", state)
				}
			} else if tc.missing {
				if state != "absent" {
					t.Fatalf("missing internal file state = %s", state)
				}
			} else {
				sum := sha256.Sum256([]byte(content))
				if state != hex.EncodeToString(sum[:]) {
					t.Fatalf("internal file state = %s", state)
				}
				rootLink := filepath.Join(t.TempDir(), "repository")
				if err := os.Symlink(root, rootLink); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				if got := hooks.NewActivityLog(rootLink, path).RecordingState(); got != state {
					t.Fatalf("symlinked root changed state: %s", got)
				}
			}
		})
	}
}
