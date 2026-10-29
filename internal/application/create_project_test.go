package application_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// memoryDirectories records what CreateProject asks of the directories
// port, in order, and hands out an in-memory ruleset and domain file.
type memoryDirectories struct {
	steps      []string
	made       string
	makeErr    error
	rulesetErr error
	discarded  string
	deleted    []string
	ruleset    recordingScaffold
	knowledge  memoryKnowledge
	// projects are the domain files of other projects, by directory.
	projects map[string]*memoryKnowledge
	// located maps a path to the project it lies in.
	located map[string]string
}

func (d *memoryDirectories) Make(dir string) (bool, error) {
	d.steps = append(d.steps, "make")
	d.made = dir
	return d.makeErr == nil, d.makeErr
}

func (d *memoryDirectories) Discard(dir string, made bool) error {
	d.steps = append(d.steps, fmt.Sprintf("discard made=%v", made))
	d.discarded = dir
	return nil
}

func (d *memoryDirectories) Ruleset(string) (application.RulesetScaffold, error) {
	d.steps = append(d.steps, "ruleset")
	if d.rulesetErr != nil {
		return nil, d.rulesetErr
	}
	return &d.ruleset, nil
}

func (d *memoryDirectories) DomainFile(dir string) (vocab.Repository, error) {
	d.steps = append(d.steps, "domain file")
	if project, ok := d.projects[dir]; ok {
		return project, nil
	}
	if d.ruleset.content == "" {
		return nil, fmt.Errorf("%s holds no rules.arclint.yaml", dir)
	}
	return &d.knowledge, nil
}

func (d *memoryDirectories) Locate(path string) (string, error) {
	if dir, ok := d.located[path]; ok {
		return dir, nil
	}
	return "", fmt.Errorf("no rules.arclint.yaml found in %s or any parent directory", path)
}

func (d *memoryDirectories) Delete(dir string) error {
	d.deleted = append(d.deleted, dir)
	return nil
}

// memoryRegistry remembers projects in memory, most recent first.
type memoryRegistry struct {
	dirs []string
	err  error
}

func (r *memoryRegistry) Directories() ([]string, error) { return slices.Clone(r.dirs), r.err }

func (r *memoryRegistry) Remember(dir string) error {
	if r.err != nil {
		return r.err
	}
	r.dirs = append([]string{dir}, slices.DeleteFunc(r.dirs, func(d string) bool { return d == dir })...)
	return nil
}

func (r *memoryRegistry) Forget(dir string) error {
	if r.err != nil {
		return r.err
	}
	r.dirs = slices.DeleteFunc(r.dirs, func(d string) bool { return d == dir })
	return nil
}

func TestCreateProjectStartsTheRulesetAndAnEmptyLanguage(t *testing.T) {
	dirs := &memoryDirectories{}
	uc, err := application.NewCreateProject(dirs, &memoryRegistry{})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "orders")
	result, err := uc.Execute(application.CreateProjectRequest{Directory: dir + string(filepath.Separator), Name: " Orders ", Languages: []string{"go", "py"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(dirs.steps, ", ") != "make, ruleset, domain file" || dirs.made != dir {
		t.Fatalf("steps = %v in %q", dirs.steps, dirs.made)
	}
	if !strings.Contains(dirs.ruleset.content, "runtime: [go, py]") || dirs.ruleset.force {
		t.Fatalf("ruleset (force %v):\n%s", dirs.ruleset.force, dirs.ruleset.content)
	}
	if dirs.knowledge.saves != 1 || dirs.knowledge.lang.Project != "Orders" || len(dirs.knowledge.lang.Contexts) != 0 {
		t.Fatalf("recorded %d times: %+v", dirs.knowledge.saves, dirs.knowledge.lang)
	}
	if result.Directory != dir || result.Project != "Orders" || result.Ruleset != "rules.arclint.yaml" || result.Source != testRecording.Path() {
		t.Fatalf("result = %+v", result)
	}
}

func TestCreateProjectNamesTheProjectForItsDirectory(t *testing.T) {
	dirs := &memoryDirectories{}
	uc, err := application.NewCreateProject(dirs, &memoryRegistry{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := uc.Execute(application.CreateProjectRequest{Directory: filepath.Join(t.TempDir(), "billing")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Project != "billing" || dirs.knowledge.lang.Project != "billing" || !strings.Contains(dirs.ruleset.content, "runtime: [go]") {
		t.Fatalf("project %q, recorded %q, ruleset:\n%s", result.Project, dirs.knowledge.lang.Project, dirs.ruleset.content)
	}
}

func TestCreateProjectRefusesBeforeMakingAnything(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "orders")
	cases := []struct {
		name   string
		req    application.CreateProjectRequest
		reason string
	}{
		{"no directory", application.CreateProjectRequest{Directory: "  "}, "name the directory"},
		{"relative directory", application.CreateProjectRequest{Directory: "orders"}, "not an absolute path"},
		{"unknown language", application.CreateProjectRequest{Directory: abs, Languages: []string{"go", "cobol"}}, "cobol"},
	}
	for _, c := range cases {
		dirs := &memoryDirectories{}
		uc, err := application.NewCreateProject(dirs, &memoryRegistry{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := uc.Execute(c.req); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.reason)
		}
		if len(dirs.steps) != 0 {
			t.Errorf("%s: asked the directories to %v", c.name, dirs.steps)
		}
	}
}

func TestCreateProjectStopsWhenTheDirectoryIsRefused(t *testing.T) {
	dirs := &memoryDirectories{makeErr: errors.New("/x/orders already holds files")}
	uc, err := application.NewCreateProject(dirs, &memoryRegistry{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.Execute(application.CreateProjectRequest{Directory: filepath.Join(t.TempDir(), "orders")})
	if err == nil || !strings.Contains(err.Error(), "already holds files") {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(dirs.steps, ", ") != "make" {
		t.Fatalf("steps after a refused directory = %v", dirs.steps)
	}
}

func TestCreateProjectTakesBackAProjectItCannotStart(t *testing.T) {
	cases := []struct {
		name  string
		dirs  *memoryDirectories
		steps string
		cause string
	}{
		{"the ruleset cannot be opened", &memoryDirectories{rulesetErr: errors.New("permission denied")},
			"make, ruleset, discard made=true", "permission denied"},
		{"the language cannot be recorded", &memoryDirectories{knowledge: memoryKnowledge{save: errors.New("disk full")}},
			"make, ruleset, domain file, discard made=true", "disk full"},
	}
	for _, c := range cases {
		uc, err := application.NewCreateProject(c.dirs, &memoryRegistry{})
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(t.TempDir(), "orders")
		_, err = uc.Execute(application.CreateProjectRequest{Directory: dir})
		if err == nil || !strings.Contains(err.Error(), c.cause) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.cause)
		}
		if got := strings.Join(c.dirs.steps, ", "); got != c.steps || c.dirs.discarded != dir {
			t.Errorf("%s: steps = %q, discarded %q; want %q of %q", c.name, got, c.dirs.discarded, c.steps, dir)
		}
	}
}

func TestCreateProjectRemembersTheProject(t *testing.T) {
	dirs, registry := &memoryDirectories{}, &memoryRegistry{dirs: []string{"/work/billing"}}
	uc, err := application.NewCreateProject(dirs, registry)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "orders")
	if _, err := uc.Execute(application.CreateProjectRequest{Directory: dir}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(registry.dirs, ", ") != dir+", /work/billing" {
		t.Fatalf("remembered %v", registry.dirs)
	}
}

func TestCreateProjectTakesBackAProjectItCannotRemember(t *testing.T) {
	dirs := &memoryDirectories{}
	uc, err := application.NewCreateProject(dirs, &memoryRegistry{err: errors.New("read-only file system")})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "orders")
	if _, err := uc.Execute(application.CreateProjectRequest{Directory: dir}); err == nil || !strings.Contains(err.Error(), "read-only file system") {
		t.Fatalf("err = %v", err)
	}
	if dirs.discarded != dir || !strings.HasSuffix(strings.Join(dirs.steps, ", "), "discard made=true") {
		t.Fatalf("steps %v, discarded %q", dirs.steps, dirs.discarded)
	}
}

func TestNewCreateProjectNeedsItsPorts(t *testing.T) {
	if _, err := application.NewCreateProject(nil, &memoryRegistry{}); err == nil {
		t.Fatal("NewCreateProject without directories accepted")
	}
	if _, err := application.NewCreateProject(&memoryDirectories{}, nil); err == nil {
		t.Fatal("NewCreateProject without a registry accepted")
	}
}
