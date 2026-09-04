package distribution

import (
	"strconv"
	"strings"
)

// CompareVersions orders two exact semantic versions as the
// specification does: numeric core first, then a pre-release sorts
// before its release, pre-release identifiers compare numerically when
// both are digits and lexically otherwise, and build metadata never
// counts. It returns -1, 0, or 1. Both inputs are versions
// PatternReference already accepted, so a malformed part orders as
// zero rather than failing.
func CompareVersions(a, b string) int {
	ca, pa := splitVersion(a)
	cb, pb := splitVersion(b)
	for i := 0; i < 3; i++ {
		if ca[i] != cb[i] {
			if ca[i] < cb[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case pa == "" && pb == "":
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	}
	return comparePreRelease(pa, pb)
}

// splitVersion returns the three numeric core parts and the
// pre-release string without build metadata.
func splitVersion(v string) ([3]int, string) {
	if plus := strings.Index(v, "+"); plus >= 0 {
		v = v[:plus]
	}
	pre := ""
	if dash := strings.Index(v, "-"); dash >= 0 {
		pre = v[dash+1:]
		v = v[:dash]
	}
	var core [3]int
	parts := strings.SplitN(v, ".", len(core))
	for i := range core {
		if i >= len(parts) {
			break
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			n = 0
		}
		core[i] = n
	}
	return core, pre
}

func comparePreRelease(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aNum := strconv.Atoi(as[i])
		bn, bNum := strconv.Atoi(bs[i])
		switch {
		case aNum == nil && bNum == nil:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		case aNum == nil:
			return -1
		case bNum == nil:
			return 1
		default:
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}
