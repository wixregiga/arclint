package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// MarshalLineList renders one finding per line for editor toolchains:
//
//	path:line: severity: message [ruleId]
//
// VS Code problemMatchers and vim's errorformat parse exactly this
// line-oriented shape; both are regex-only and cannot read JSON. A
// violation without a line anchor prints 0, which editors clamp to the
// top of the file — no line is ever invented.
func MarshalLineList(vs []Violation) []byte {
	var b strings.Builder
	for _, v := range vs {
		fmt.Fprintf(&b, "%s:%d: %s: %s [%s]\n", v.Path, v.LineValue(), v.Severity, v.Message, v.RuleID)
	}
	return []byte(b.String())
}

// SARIF 2.1.0 subset: exactly the properties GitHub code scanning and
// the SARIF Viewer consume. Everything arclint-specific (contract,
// blame, capability, fixHint) rides in the result property bag rather
// than inventing SARIF fields.

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version,omitempty"`
	Rules   []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID string `json:"id"`
}

type sarifResult struct {
	RuleID              string             `json:"ruleId"`
	Level               string             `json:"level"`
	Message             sarifMessage       `json:"message"`
	Locations           []sarifLocation    `json:"locations"`
	PartialFingerprints map[string]string  `json:"partialFingerprints,omitempty"`
	Suppressions        []sarifSuppression `json:"suppressions,omitempty"`
	Properties          map[string]string  `json:"properties,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

type sarifSuppression struct {
	Kind          string `json:"kind"`
	Justification string `json:"justification,omitempty"`
}

// sarifLevel maps arclint severities onto the SARIF level vocabulary.
func sarifLevel(s Severity) string {
	switch s {
	case SeverityWarn:
		return "warning"
	case SeverityInfo:
		return "note"
	default:
		return "error"
	}
}

// fingerprint identifies a finding across commits: rule, path, and
// message, deliberately excluding the line so pure line shifts do not
// reopen findings.
func fingerprint(v Violation) string {
	sum := sha256.Sum256([]byte(v.RuleID + "|" + v.Path + "|" + v.Message))
	return hex.EncodeToString(sum[:8])
}

// MarshalSARIF encodes violations as one SARIF 2.1.0 run. Suppressed
// findings (present only under --show-suppressed) carry a suppressions
// block with the except reason, so GitHub shows them as suppressed
// instead of open.
func MarshalSARIF(vs []Violation, toolVersion string) ([]byte, error) {
	seen := map[string]bool{}
	var rules []sarifRule
	results := make([]sarifResult, 0, len(vs))
	for _, v := range vs {
		if !seen[v.RuleID] {
			seen[v.RuleID] = true
			rules = append(rules, sarifRule{ID: v.RuleID})
		}
		loc := sarifPhysicalLocation{ArtifactLocation: sarifArtifactLocation{URI: v.Path}}
		if v.Line != nil {
			loc.Region = &sarifRegion{StartLine: *v.Line}
		}
		props := map[string]string{
			"contract": string(v.Contract),
			"blame":    string(v.Blame),
		}
		if v.Capability != "" {
			props["capability"] = v.Capability
		}
		if v.FixHint != "" {
			props["fixHint"] = v.FixHint
		}
		result := sarifResult{
			RuleID:              v.RuleID,
			Level:               sarifLevel(v.Severity),
			Message:             sarifMessage{Text: v.Message},
			Locations:           []sarifLocation{{PhysicalLocation: loc}},
			PartialFingerprints: map[string]string{"arclintFingerprint/v1": fingerprint(v)},
			Properties:          props,
		}
		if v.Suppressed {
			result.Suppressions = []sarifSuppression{{Kind: "external", Justification: v.SuppressedReason}}
		}
		results = append(results, result)
	}
	if rules == nil {
		rules = []sarifRule{}
	}
	log := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: "arclint", Version: toolVersion, Rules: rules}},
			Results: results,
		}},
	}
	return json.MarshalIndent(log, "", "  ")
}
