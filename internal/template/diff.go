package template

import (
	"bytes"
	"fmt"
	"strings"
)

// UnifiedDiff produces a unified diff (3 lines of context) between a (disk)
// and b (new render) labeled a/<path> and b/<path>. Returns "" when equal.
// Line-based LCS; template-scale files, so the quadratic table is fine.
func UnifiedDiff(path string, a, b []byte) string {
	if bytes.Equal(a, b) {
		return ""
	}
	ops := diffOps(splitDiffLines(a), splitDiffLines(b))

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", path, path)

	// Precompute the 1-based a/b line numbers at each op.
	type pos struct{ a, b int }
	positions := make([]pos, len(ops)+1)
	ai, bi := 1, 1
	for i, o := range ops {
		positions[i] = pos{ai, bi}
		switch o.kind {
		case ' ':
			ai++
			bi++
		case '-':
			ai++
		case '+':
			bi++
		}
	}
	positions[len(ops)] = pos{ai, bi}

	const ctx = 3
	i := 0
	for i < len(ops) {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		hunkStart := max(0, i-ctx)
		// Extend the hunk while gaps of unchanged lines stay within 2*ctx.
		lastChange := i
		j := i
		for j < len(ops) {
			if ops[j].kind != ' ' {
				lastChange = j
				j++
				continue
			}
			k := j
			for k < len(ops) && ops[k].kind == ' ' {
				k++
			}
			if k < len(ops) && k-j <= 2*ctx {
				j = k
				continue
			}
			break
		}
		hunkEnd := min(len(ops), lastChange+ctx+1)

		aCount, bCount := 0, 0
		for _, o := range ops[hunkStart:hunkEnd] {
			switch o.kind {
			case ' ':
				aCount++
				bCount++
			case '-':
				aCount++
			case '+':
				bCount++
			}
		}
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", positions[hunkStart].a, aCount, positions[hunkStart].b, bCount)
		for _, o := range ops[hunkStart:hunkEnd] {
			sb.WriteByte(o.kind)
			sb.WriteString(o.text)
			sb.WriteByte('\n')
		}
		i = hunkEnd
	}
	return sb.String()
}

type diffOp struct {
	kind byte // ' ', '-', '+'
	text string
}

func diffOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	if n*m > 2_000_000 {
		// Degenerate fallback for huge files: whole-file replace.
		ops := make([]diffOp, 0, n+m)
		for _, l := range a {
			ops = append(ops, diffOp{'-', l})
		}
		for _, l := range b {
			ops = append(ops, diffOp{'+', l})
		}
		return ops
	}
	// dp[i][j] = LCS length of a[i:] vs b[j:].
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}

// splitDiffLines splits content into lines without trailing newlines. The
// no-newline-at-EOF distinction is not preserved in the diff rendering;
// equality checks elsewhere use raw bytes, so drift detection is exact.
func splitDiffLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
