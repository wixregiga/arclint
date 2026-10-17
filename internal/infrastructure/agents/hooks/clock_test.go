package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/agent"
)

// The fix for issue #170 rests on this: a write made after a file clock
// reading is never stamped before it, while a process clock reading taken
// before the same write often comes later than its stamp.
func TestAWriteAfterAFileClockReadingIsNeverStampedBeforeIt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "probe")
	for index := range 2000 {
		read := clockOf(t, root)
		if err := os.WriteFile(path, []byte(strconv.Itoa(index)), 0o600); err != nil {
			t.Fatal(err)
		}
		if stamped := fingerprintOf(path).at(); stamped.Before(read) {
			t.Fatalf("write %d stamped %v, before the file clock read %v", index, stamped, read)
		}
	}
}

// A change stamped in the same tick as a check may have come after it, so
// it counts as after it and stays unverified.
func TestAChangeInTheSameTickAsACheckCountsAfterIt(t *testing.T) {
	root := gitRepository(t, map[string]string{"rules.arclint.yaml": "zones:\n  app: internal/**\n", "internal/a.go": "package a\n"})
	store := NewProgressStore(root, recording)
	observe(t, store, "s")
	pause(t)
	writeFile(t, root, "internal/a.go", "package a // changed\n")
	stamped := fingerprintOf(filepath.Join(root, "internal/a.go")).at()
	appendLine(t, root, loggedActivity{Kind: string(agent.CheckRan), At: stamped})
	pause(t)
	want := []agent.Activity{{Kind: agent.CheckRan}, {Kind: agent.FilesChanged, Paths: []string{"internal/a.go"}}}
	if seen := observe(t, store, "s"); !reflect.DeepEqual(seen.recorded, want) {
		t.Fatalf("recorded %+v, want %+v", seen.recorded, want)
	}
}

// A change in the same tick as either end of a time between turns may have
// been the agent's, so the idle time never excludes it.
func TestAnIdleTimeKeepsChangesInTheTickOfEitherEnd(t *testing.T) {
	since := time.Unix(100, 0)
	until := time.Unix(200, 0)
	idle := idleTime{Since: since, Until: until}
	for at, excluded := range map[time.Time]bool{
		since:                       false,
		until:                       false,
		since.Add(time.Nanosecond):  true,
		until.Add(-time.Nanosecond): true,
		until.Add(time.Nanosecond):  false,
	} {
		if got := idle.excludes(at); got != excluded {
			t.Errorf("excludes(%v) = %t, want %t", at, got, excluded)
		}
	}
	if (idleTime{Since: since}).excludes(since.Add(time.Second)) {
		t.Error("an idle time still open excluded a change")
	}
	if !(idleTime{Until: until}).excludes(time.Unix(1, 0)) {
		t.Error("an idle time from always kept a change before its end")
	}
}

// appendLine writes one activity log line as a command does, under the
// log's lock.
func appendLine(t *testing.T, root string, logged loggedActivity) {
	t.Helper()
	file, err := os.OpenFile(filepath.Join(root, activityLog), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	unlock, err := lockFile(file)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()
	line, err := json.Marshal(logged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
}
