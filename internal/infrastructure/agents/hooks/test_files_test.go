package hooks

import "testing"

func TestTestConventionsTellTestsApart(t *testing.T) {
	for name, want := range map[string]bool{
		"internal/domain/agent/guide_test.go":      true,
		"internal/domain/agent/guide.go":           false,
		"web/src/features/login/login.test.ts":     true,
		"web/src/features/login/login.spec.tsx":    true,
		"web/src/features/login/__tests__/form.ts": true,
		"web/src/features/login/login.ts":          false,
		"docs/api.spec.yaml":                       false,
		"pkg/billing/test_invoice.py":              true,
		"pkg/billing/invoice_test.py":              true,
		"pkg/conftest.py":                          true,
		"pkg/billing/invoice.py":                   false,
		"tests/e2e/flow.go":                        true,
		"internal/rule/testdata/fixture.go":        true,
		".arclint/tests/zones.yaml":                true,
		"contest/entry.go":                         false,
		"latest.go":                                false,
	} {
		if got := (TestConventions{}).Test(name); got != want {
			t.Errorf("Test(%q) = %t, want %t", name, got, want)
		}
	}
}
