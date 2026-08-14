package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func sample() []Violation {
	return []Violation{
		{RuleID: "ddd:ARCH-001", Contract: ContractConsumes, Blame: BlameConsumer,
			Severity: SeverityError, Capability: "exact", Path: "domain/book.go",
			Line: IntPtr(3), Message: "domain imports application", FixHint: "invert the dependency"},
		{RuleID: "deps.acyclic", Contract: ContractInvariant, Blame: BlameProvider,
			Severity: SeverityWarn, Path: "app/wire.go",
			Message: "cycle app -> domain -> app"},
	}
}

func TestMarshalLineList(t *testing.T) {
	got := string(MarshalLineList(sample()))
	want := "domain/book.go:3: error: domain imports application [ddd:ARCH-001]\n" +
		"app/wire.go:0: warn: cycle app -> domain -> app [deps.acyclic]\n"
	if got != want {
		t.Errorf("line format:\ngot  %q\nwant %q", got, want)
	}
}

func TestMarshalSARIF(t *testing.T) {
	vs := sample()
	vs = append(vs, Violation{RuleID: "ddd:ARCH-001", Severity: SeverityError,
		Path: "domain/legacy.go", Message: "grandfathered import",
		Suppressed: true, SuppressedReason: "migration scheduled"})
	data, err := MarshalSARIF(vs, "0.1.0 (abc1234, 2026-08-14)")
	if err != nil {
		t.Fatal(err)
	}
	var log struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Level     string `json:"level"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region *struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
				PartialFingerprints map[string]string `json:"partialFingerprints"`
				Suppressions        []struct {
					Kind          string `json:"kind"`
					Justification string `json:"justification"`
				} `json:"suppressions"`
				Properties map[string]string `json:"properties"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatalf("sarif output is not valid JSON: %v", err)
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("version/runs: %s", data)
	}
	run := log.Runs[0]
	if run.Tool.Driver.Name != "arclint" || len(run.Tool.Driver.Rules) != 2 {
		t.Errorf("driver: %+v", run.Tool.Driver)
	}
	if len(run.Results) != 3 {
		t.Fatalf("results: %d", len(run.Results))
	}
	first := run.Results[0]
	if first.Level != "error" ||
		first.Locations[0].PhysicalLocation.ArtifactLocation.URI != "domain/book.go" ||
		first.Locations[0].PhysicalLocation.Region.StartLine != 3 ||
		first.Properties["fixHint"] != "invert the dependency" {
		t.Errorf("first result: %+v", first)
	}
	if run.Results[1].Level != "warning" || run.Results[1].Locations[0].PhysicalLocation.Region != nil {
		t.Errorf("unanchored warn result: %+v", run.Results[1])
	}
	sup := run.Results[2]
	if len(sup.Suppressions) != 1 || sup.Suppressions[0].Kind != "external" ||
		sup.Suppressions[0].Justification != "migration scheduled" {
		t.Errorf("suppressed result: %+v", sup)
	}
	// Fingerprints are stable across encodes and line moves: the same
	// finding two lines lower keeps its identity.
	fp := first.PartialFingerprints["arclintFingerprint/v1"]
	if fp == "" {
		t.Fatal("missing fingerprint")
	}
	moved := sample()[0]
	moved.Line = IntPtr(5)
	if got := Fingerprint(moved); got != fp {
		t.Errorf("fingerprint changed on line move: %s vs %s", got, fp)
	}
	if strings.Contains(string(data), "\"line\"") {
		t.Error("sarif output leaked the internal violation shape")
	}
}
