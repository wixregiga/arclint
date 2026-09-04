package jsonbaseline_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/baseline"
	jsonbaseline "github.com/wixregiga/arclint/internal/infrastructure/baseline/json"
)

func snapshot(t *testing.T) baseline.Snapshot {
	t.Helper()
	entry, err := baseline.NewEntry("t/p:m/snake", "m/BadName.go", "file name violates naming rule", 2)
	if err != nil {
		t.Fatalf("NewEntry: %v", err)
	}
	s, err := baseline.New([]string{"t/p:m/snake"}, []baseline.Entry{entry}, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestRoundtripIsDeterministic(t *testing.T) {
	root := t.TempDir()
	store, err := jsonbaseline.NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Write(snapshot(t)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(jsonbaseline.Path)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := store.Write(snapshot(t)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(jsonbaseline.Path)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("identical snapshots produced different bytes")
	}

	loaded, present, err := store.Load()
	if err != nil || !present {
		t.Fatalf("Load = (%v, %v)", present, err)
	}
	entries := loaded.Entries()
	if len(entries) != 1 || entries[0].Count() != 2 || entries[0].RuleID() != "t/p:m/snake" {
		t.Errorf("loaded entries = %+v", entries)
	}
	if rules := loaded.CapturedRules(); len(rules) != 1 || rules[0] != "t/p:m/snake" {
		t.Errorf("loaded captured rules = %v", rules)
	}
}

func TestLoadDistinguishesAbsenceFromCorruption(t *testing.T) {
	root := t.TempDir()
	store, err := jsonbaseline.NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, present, err := store.Load(); err != nil || present {
		t.Errorf("absent baseline must load as (false, nil), got (%v, %v)", present, err)
	}

	target := filepath.Join(root, filepath.FromSlash(jsonbaseline.Path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(target, []byte("{broken"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, _, err := store.Load(); err == nil {
		t.Errorf("a corrupted policy file must not silently disable itself")
	}
	if err := os.WriteFile(target, []byte(`{"version": 99, "capturedRules": [], "findings": []}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, _, err := store.Load(); err == nil {
		t.Errorf("a wrong-version baseline must be a configuration error")
	}
}
