package registry

import (
	"fmt"
	"strconv"
	"strings"
)

type version struct {
	major, minor, patch int
}

func parseSemver(s string) (version, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "v"))
	if s == "" {
		return version{}, fmt.Errorf("empty version")
	}
	parts := strings.Split(s, ".")
	if len(parts) < 1 {
		return version{}, fmt.Errorf("invalid semver %q", s)
	}
	nums := [3]int{}
	for i := 0; i < 3 && i < len(parts); i++ {
		p := parts[i]
		if dash := strings.IndexByte(p, '-'); dash >= 0 {
			p = p[:dash]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return version{}, fmt.Errorf("invalid semver %q", s)
		}
		nums[i] = n
	}
	return version{nums[0], nums[1], nums[2]}, nil
}

func cmpVer(a, b version) int {
	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return -1
		}
		return 1
	}
	return 0
}

// CompatibleEngine reports whether engineVersion satisfies a simple range.
// Supported: "*", ">=x.y.z", ">x.y.z", "<=x.y.z", "<x.y.z", "=x.y.z", "x.y.z".
func CompatibleEngine(engineVersion, rangeExpr string) bool {
	rangeExpr = strings.TrimSpace(rangeExpr)
	if rangeExpr == "" || rangeExpr == "*" {
		return true
	}
	ev, err := parseSemver(engineVersion)
	if err != nil {
		return false
	}
	op := ">="
	rest := rangeExpr
	switch {
	case strings.HasPrefix(rangeExpr, ">="):
		op, rest = ">=", strings.TrimSpace(rangeExpr[2:])
	case strings.HasPrefix(rangeExpr, "<="):
		op, rest = "<=", strings.TrimSpace(rangeExpr[2:])
	case strings.HasPrefix(rangeExpr, ">"):
		op, rest = ">", strings.TrimSpace(rangeExpr[1:])
	case strings.HasPrefix(rangeExpr, "<"):
		op, rest = "<", strings.TrimSpace(rangeExpr[1:])
	case strings.HasPrefix(rangeExpr, "="):
		op, rest = "=", strings.TrimSpace(rangeExpr[1:])
	default:
		op, rest = "=", rangeExpr
	}
	rv, err := parseSemver(rest)
	if err != nil {
		return false
	}
	c := cmpVer(ev, rv)
	switch op {
	case ">=":
		return c >= 0
	case ">":
		return c > 0
	case "<=":
		return c <= 0
	case "<":
		return c < 0
	default:
		return c == 0
	}
}

// VersionNewer reports whether a is semver-greater than b.
func VersionNewer(a, b string) bool {
	va, errA := parseSemver(a)
	vb, errB := parseSemver(b)
	if errA != nil || errB != nil {
		return a != b && a > b
	}
	return cmpVer(va, vb) > 0
}
