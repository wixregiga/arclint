package application_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// schemaPublisher is the shape both schema use cases share.
type schemaPublisher interface {
	Render() ([]byte, error)
	Execute(dir string) (bool, string, error)
}

// publishedSchemas enumerates each schema use case with the file it
// publishes; every committed copy (the release copy under docs/schemas
// and the dogfood copy under the project's schema directory) must be
// byte-identical to the generator's output.
func publishedSchemas(t *testing.T, writer application.ArtifactWriter) map[string]schemaPublisher {
	t.Helper()
	ruleSchema, err := application.NewPublishRuleSchema(writer)
	if err != nil {
		t.Fatalf("NewPublishRuleSchema: %v", err)
	}
	domainSchema, err := application.NewPublishDomainSchema(writer)
	if err != nil {
		t.Fatalf("NewPublishDomainSchema: %v", err)
	}
	return map[string]schemaPublisher{
		rule.SchemaFileName:  ruleSchema,
		vocab.SchemaFileName: domainSchema,
	}
}

// On failure: regenerate the committed schemas via make schemas, or fix
// the generator; never edit the committed copies by hand.
func TestPublishSchemaRenderMatchesCommittedCopies(t *testing.T) {
	root := repoRoot(t)
	for filename, uc := range publishedSchemas(t, stubArtifactWriter{}) {
		got, err := uc.Render()
		if err != nil {
			t.Fatalf("%s: Render: %v", filename, err)
		}
		for _, dir := range []string{filepath.Join("docs", "schemas"), application.SchemaDirectory} {
			path := filepath.Join(root, dir, filename)
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read committed %s: %v", path, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s drifted from the generator; run make schemas, or fix the generator; never edit the committed copies by hand", path)
			}
		}
	}
}

type recordingArtifactWriter struct {
	dir      string
	filename string
	content  []byte
}

func (w *recordingArtifactWriter) Write(dir, filename string, content []byte) (bool, string, error) {
	w.dir, w.filename, w.content = dir, filename, content
	return true, filepath.Join(dir, filename), nil
}

func TestPublishSchemaExecuteDefaultsToSchemaDirectory(t *testing.T) {
	writer := &recordingArtifactWriter{}
	for filename, uc := range publishedSchemas(t, writer) {
		changed, path, err := uc.Execute("")
		if err != nil {
			t.Fatalf("%s: Execute: %v", filename, err)
		}
		if !changed {
			t.Fatalf("%s: Execute reported no change from a writer that wrote", filename)
		}
		if writer.dir != application.SchemaDirectory {
			t.Fatalf("%s: default dir = %q, want %q", filename, writer.dir, application.SchemaDirectory)
		}
		if writer.filename != filename {
			t.Fatalf("filename = %q, want %q", writer.filename, filename)
		}
		if want := filepath.Join(application.SchemaDirectory, filename); path != want {
			t.Fatalf("%s: path = %q, want %q", filename, path, want)
		}
		want, err := uc.Render()
		if err != nil {
			t.Fatalf("%s: Render: %v", filename, err)
		}
		if !bytes.Equal(writer.content, want) {
			t.Fatalf("%s: Execute wrote different bytes than Render", filename)
		}
	}
}

func TestPublishSchemaExecuteHonoursExplicitDirectory(t *testing.T) {
	writer := &recordingArtifactWriter{}
	for filename, uc := range publishedSchemas(t, writer) {
		if _, _, err := uc.Execute("docs/schemas"); err != nil {
			t.Fatalf("%s: Execute: %v", filename, err)
		}
		if writer.dir != "docs/schemas" {
			t.Fatalf("%s: dir = %q, want docs/schemas", filename, writer.dir)
		}
	}
}

func TestPublishSchemaRequiresWriter(t *testing.T) {
	if _, err := application.NewPublishRuleSchema(nil); err == nil {
		t.Fatal("NewPublishRuleSchema accepted a nil writer")
	}
	if _, err := application.NewPublishDomainSchema(nil); err == nil {
		t.Fatal("NewPublishDomainSchema accepted a nil writer")
	}
}
