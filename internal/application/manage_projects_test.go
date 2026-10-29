package application_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// projectRecording is a project's domain file recording name and description.
func projectRecording(t *testing.T, name, description string) *memoryKnowledge {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage(name, description, []vocab.BoundedContext{{Name: "ordering", Definition: "Taking orders."}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &memoryKnowledge{lang: lang, found: true}
}

func TestListProjectsListsTheServedProjectFirstThenTheRemembered(t *testing.T) {
	dirs := &memoryDirectories{projects: map[string]*memoryKnowledge{
		"/work/orders":  projectRecording(t, "orders", "Taking orders."),
		"/work/billing": projectRecording(t, "billing", ""),
		"/work/empty":   {},
	}}
	registry := &memoryRegistry{dirs: []string{"/work/billing", "/work/orders", "/work/gone", "/work/empty"}}
	uc, err := application.NewListProjects(dirs, registry)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := uc.Execute("/work/orders")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range listed {
		got = append(got, p.Directory+"="+p.Name+"|"+p.Problem)
	}
	want := []string{
		"/work/orders=orders|",
		"/work/billing=billing|",
		"/work/gone=|/work/gone holds no rules.arclint.yaml",
		"/work/empty=|" + testRecording.Path() + " records no language yet",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("listed:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if listed[0].Description != "Taking orders." {
		t.Fatalf("description = %q", listed[0].Description)
	}

	// A registry that cannot be read still lists the project served.
	registry.err = errors.New("permission denied")
	listed, err = uc.Execute("/work/orders")
	if err == nil || !strings.Contains(err.Error(), "permission denied") || len(listed) != 1 || listed[0].Name != "orders" {
		t.Fatalf("listed %+v, err %v", listed, err)
	}
}

func TestOpenProjectRemembersTheProjectAPathLiesIn(t *testing.T) {
	dirs := &memoryDirectories{located: map[string]string{"/work/orders/internal/domain": "/work/orders"}}
	registry := &memoryRegistry{dirs: []string{"/work/billing"}}
	uc, err := application.NewOpenProject(dirs, registry)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := uc.Execute(" /work/orders/internal/domain/ ")
	if err != nil || dir != "/work/orders" {
		t.Fatalf("opened %q, err %v", dir, err)
	}
	if strings.Join(registry.dirs, ",") != "/work/orders,/work/billing" {
		t.Fatalf("remembered %v", registry.dirs)
	}
	for path, reason := range map[string]string{"": "name a path", "orders": "not an absolute path", "/elsewhere": "no rules.arclint.yaml"} {
		if _, err := uc.Execute(path); err == nil || !strings.Contains(err.Error(), reason) {
			t.Errorf("open %q: err = %v, want %q", path, err, reason)
		}
	}
	if len(registry.dirs) != 2 {
		t.Fatalf("a refused open was remembered: %v", registry.dirs)
	}
}

func TestUpdateProjectRecordsNameAndDescriptionInOneWrite(t *testing.T) {
	orders := projectRecording(t, "orders", "Taking orders.")
	dirs := &memoryDirectories{projects: map[string]*memoryKnowledge{"/work/orders": orders}}
	uc, err := application.NewUpdateProject(dirs)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := uc.Execute(application.UpdateProjectRequest{Directory: "/work/orders", Name: " Ordering ", Description: " Taking and fulfilling orders. "})
	if err != nil {
		t.Fatal(err)
	}
	if orders.saves != 1 || orders.lang.Project != "Ordering" || orders.lang.Description != "Taking and fulfilling orders." || len(orders.lang.Contexts) != 1 {
		t.Fatalf("saved %d times: %+v", orders.saves, orders.lang)
	}
	if summary.Name != "Ordering" || summary.Description != "Taking and fulfilling orders." {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := uc.Execute(application.UpdateProjectRequest{Directory: "/work/orders", Name: "Ordering", Description: "Taking and fulfilling orders."}); err != nil || orders.saves != 1 {
		t.Fatalf("an unchanged update wrote: saves %d, err %v", orders.saves, err)
	}
	cases := []struct{ name, description, reason string }{
		{" ", "", "project is empty"},
		{"Ordering", strings.Repeat("word ", 101), "100"},
	}
	for _, c := range cases {
		if _, err := uc.Execute(application.UpdateProjectRequest{Directory: "/work/orders", Name: c.name, Description: c.description}); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("name %q: err = %v, want %q", c.name, err, c.reason)
		}
	}
	if orders.saves != 1 || orders.lang.Project != "Ordering" {
		t.Fatalf("a refused update wrote: %+v", orders.lang)
	}
}

func TestRemoveProjectForgetsAndDeletesOnlyWhenConfirmed(t *testing.T) {
	dirs := &memoryDirectories{located: map[string]string{"/work/orders": "/work/orders", "/work/nested": "/work"}}
	registry := &memoryRegistry{dirs: []string{"/work/orders", "/work/billing", "/work/nested"}}
	uc, err := application.NewRemoveProject(dirs, registry)
	if err != nil {
		t.Fatal(err)
	}
	refusals := []struct {
		req    application.RemoveProjectRequest
		reason string
	}{
		{application.RemoveProjectRequest{Directory: "/work/unknown"}, "not on the list"},
		{application.RemoveProjectRequest{Directory: "/work/orders", DeleteFiles: true}, `type "orders"`},
		{application.RemoveProjectRequest{Directory: "/work/orders", DeleteFiles: true, Confirmation: "Orders"}, `type "orders"`},
		{application.RemoveProjectRequest{Directory: "/work/nested", DeleteFiles: true, Confirmation: "nested"}, "not a project's directory"},
	}
	for _, c := range refusals {
		if err := uc.Execute(c.req); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%+v: err = %v, want %q", c.req, err, c.reason)
		}
	}
	if len(dirs.deleted) != 0 || len(registry.dirs) != 3 {
		t.Fatalf("a refusal deleted %v or forgot: %v", dirs.deleted, registry.dirs)
	}
	if err := uc.Execute(application.RemoveProjectRequest{Directory: "/work/billing"}); err != nil {
		t.Fatal(err)
	}
	if len(dirs.deleted) != 0 || strings.Join(registry.dirs, ",") != "/work/orders,/work/nested" {
		t.Fatalf("forgetting deleted %v; remembered %v", dirs.deleted, registry.dirs)
	}
	if err := uc.Execute(application.RemoveProjectRequest{Directory: "/work/orders", DeleteFiles: true, Confirmation: "orders"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(dirs.deleted, ",") != "/work/orders" || strings.Join(registry.dirs, ",") != "/work/nested" {
		t.Fatalf("deleted %v; remembered %v", dirs.deleted, registry.dirs)
	}
}
