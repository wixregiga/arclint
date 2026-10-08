package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/workflow"
)

type observation struct {
	progress workflow.Progress
	recorded []workflow.Activity
}

func observe(t *testing.T, store *ProgressStore, session string) observation {
	t.Helper()
	var seen observation
	if err := store.Update(session, func(progress workflow.Progress, recorded []workflow.Activity) workflow.Progress {
		seen = observation{progress: progress, recorded: recorded}
		return progress
	}); err != nil {
		t.Fatal(err)
	}
	return seen
}

func addContext(path string) func(workflow.Progress, []workflow.Activity) workflow.Progress {
	return func(progress workflow.Progress, _ []workflow.Activity) workflow.Progress {
		progress.Zones = append(progress.Zones, path)
		return progress
	}
}

func TestProgressStoreKeepsSessionsApart(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	for _, step := range []struct{ session, path string }{{"a", "x"}, {"b", "y"}, {"a", "z"}} {
		if err := store.Update(step.session, addContext(step.path)); err != nil {
			t.Fatal(err)
		}
	}
	if seen := observe(t, store, "a"); !reflect.DeepEqual(seen.progress.Zones, []string{"x", "z"}) {
		t.Fatalf("session a progress: %+v", seen.progress)
	}
}

func TestProgressStoreReadsRecordedActivityOncePerSession(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	log := NewActivityLog(root, recording)
	if err := log.Record(workflow.Activity{Kind: workflow.CheckRan}); err != nil {
		t.Fatal(err)
	}
	if entries, _ := filepath.Glob(filepath.Join(root, progressDirectory, "*")); len(entries) != 0 {
		t.Fatalf("recorded without the hooks' progress directory: %v", entries)
	}
	observe(t, store, "a")
	context := workflow.Activity{Kind: workflow.ContextObtained, Paths: []string{"internal"}}
	if err := log.Record(context); err != nil {
		t.Fatal(err)
	}
	if seen := observe(t, store, "a"); !reflect.DeepEqual(seen.recorded, []workflow.Activity{context}) {
		t.Fatalf("first read: %+v", seen.recorded)
	}
	if seen := observe(t, store, "a"); len(seen.recorded) != 0 {
		t.Fatalf("recorded activity read twice: %+v", seen.recorded)
	}
	if seen := observe(t, store, "b"); len(seen.recorded) != 0 {
		t.Fatalf("a new session read earlier activity: %+v", seen.recorded)
	}
}

func TestProgressStoreLeavesAPartialLineForLater(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	observe(t, store, "s")
	file, err := os.OpenFile(filepath.Join(root, activityLog), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.WriteString(`{"kind":"check"}` + "\n" + `{"kind":"con`); err != nil {
		t.Fatal(err)
	}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, []workflow.Activity{{Kind: workflow.CheckRan}}) {
		t.Fatalf("complete lines: %+v", seen.recorded)
	}
	if _, err := file.WriteString(`text"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, []workflow.Activity{{Kind: workflow.ContextObtained}}) {
		t.Fatalf("the finished line: %+v", seen.recorded)
	}
}

func TestProgressStoreStartsOverFromAnUnreadableRecord(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	if err := store.Update("s", addContext("x")); err != nil {
		t.Fatal(err)
	}
	records, err := filepath.Glob(filepath.Join(root, progressDirectory, "*.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("progress records: %v %v", records, err)
	}
	if err := os.WriteFile(records[0], []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.progress, workflow.Progress{}) {
		t.Fatalf("unreadable record was not started over: %+v", seen.progress)
	}
}

func TestProgressStorePrunesStaleSessionsAndTheirLog(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	observe(t, store, "old")
	if err := NewActivityLog(root, recording).Record(workflow.Activity{Kind: workflow.CheckRan}); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-2 * staleSession)
	records, err := filepath.Glob(filepath.Join(root, progressDirectory, "*.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("records: %v %v", records, err)
	}
	if err := os.Chtimes(records[0], stale, stale); err != nil {
		t.Fatal(err)
	}
	observe(t, store, "new")
	remaining, err := filepath.Glob(filepath.Join(root, progressDirectory, "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range remaining {
		if path == records[0] || path == records[0]+".lock" || path == filepath.Join(root, activityLog) {
			t.Fatalf("stale state kept: %v", remaining)
		}
	}
}

func TestProgressStoreKeepsAStaleSessionThatHoldsItsLock(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	observe(t, store, "old")
	records, err := filepath.Glob(filepath.Join(root, progressDirectory, "*.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("records: %v %v", records, err)
	}
	stale := time.Now().Add(-2 * staleSession)
	if err := os.Chtimes(records[0], stale, stale); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(records[0]+".lock", os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	unlock, err := lockFile(lock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()
	observe(t, store, "new")
	for _, path := range []string{records[0], records[0] + ".lock"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("a session holding its lock lost %s: %v", path, err)
		}
	}
}

func TestProgressStoreIgnoresARecordWithoutACursor(t *testing.T) {
	root := t.TempDir()
	store := NewProgressStore(root)
	observe(t, store, "s")
	if err := NewActivityLog(root, recording).Record(workflow.Activity{Kind: workflow.CheckRan}); err != nil {
		t.Fatal(err)
	}
	records, err := filepath.Glob(filepath.Join(root, progressDirectory, "*.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("records: %v %v", records, err)
	}
	if err := os.WriteFile(records[0], []byte(`{"Progress":{"DomainRecorded":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if seen := observe(t, store, "s"); seen.progress.DomainRecorded || len(seen.recorded) != 0 {
		t.Fatalf("a cursorless record was trusted: %+v", seen)
	}
}

func TestProgressStoreSerializesConcurrentUpdates(t *testing.T) {
	store := NewProgressStore(t.TempDir())
	var group sync.WaitGroup
	for index := range 24 {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := store.Update("s", addContext(fmt.Sprint(index))); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if seen := observe(t, store, "s"); len(seen.progress.Zones) != 24 {
		t.Fatalf("lost concurrent updates: %d of 24", len(seen.progress.Zones))
	}
}
