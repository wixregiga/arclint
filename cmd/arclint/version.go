package main

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// buildVersion renders the version line: the product version stays what
// -X main.version set (0.1.0 until a compat commitment exists), and the
// build is distinguished by the VCS settings Go stamps automatically
// since 1.18: "0.1.0 (abc1234, dirty, 2026-08-14)". Builds without VCS
// data (source archives, -buildvcs=false, go run) fall back to the bare
// product version.
func buildVersion(version string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	var revision, timestamp string
	dirty := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			timestamp = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	return formatVersion(version, revision, timestamp, dirty)
}

// formatVersion is the pure composition, separated so tests can pin the
// format without controlling the test binary's own build info.
func formatVersion(version, revision, timestamp string, dirty bool) string {
	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	parts := []string{revision}
	if dirty {
		parts = append(parts, "dirty")
	}
	if len(timestamp) >= len("2006-01-02") {
		parts = append(parts, timestamp[:len("2006-01-02")])
	}
	return fmt.Sprintf("%s (%s)", version, strings.Join(parts, ", "))
}
