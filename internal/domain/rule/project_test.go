package rule

import (
	"errors"
	"path/filepath"
	"testing"
)

// A Project always has a name: a rename to nothing is refused, and so
// is a default taken from a directory with no name of its own.
func TestProjectIsNeverNameless(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	for _, directory := range []string{root, "."} {
		if p, err := NewProject(directory, ""); !errors.Is(err, ErrProjectNameless) {
			t.Errorf("NewProject(%q, \"\") = %+v, %v; want ErrProjectNameless", directory, p, err)
		}
	}
	if p, err := NewProject(root, " shop "); err != nil || p.Name() != "shop" {
		t.Errorf("a named Project at the root = %+v, %v; want shop", p, err)
	}
	p, err := NewProject(filepath.Join(root, "repos", "shop"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "  "} {
		if q, err := p.Renamed(name); !errors.Is(err, ErrProjectNameless) {
			t.Errorf("Renamed(%q) = %+v, %v; want ErrProjectNameless", name, q, err)
		}
	}
	if _, err := NewProject("", "shop"); err == nil {
		t.Error("a Project without a directory was built")
	}
}

// A Project whose domain file records no name is named for the
// directory holding the ruleset, and a rename keeps that directory.
func TestProjectIsNamedForItsDirectory(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "box office")
	p, err := NewProject(directory+string(filepath.Separator), "")
	if err != nil || p.Name() != "box office" {
		t.Fatalf("NewProject = %+v, %v; want box office", p, err)
	}
	q, err := p.Renamed(" boxoffice ")
	if err != nil || q.Name() != "boxoffice" || q.directory != p.directory {
		t.Fatalf("Renamed = %+v, %v; want boxoffice in %q", q, err, p.directory)
	}
}
