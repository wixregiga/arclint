package agent

import (
	"reflect"
	"testing"
)

func TestObtainedContextShowsOwningAndNamedZones(t *testing.T) {
	owners := map[string][]string{
		"internal/domain/order/order.go": {"domain", "ordering"},
		"cmd/main.go":                    {"composition"},
		"docs/notes.md":                  nil,
	}
	owning := func(path string) []string { return owners[path] }
	for name, tc := range map[string]struct {
		paths, named, want []string
		everyZone          bool
	}{
		"paths and names":     {[]string{"internal/domain/order/order.go", "cmd/main.go"}, []string{"ordering", "web"}, []string{"composition", "domain", "ordering", "web"}, false},
		"a path no Zone owns": {[]string{"docs/notes.md"}, nil, nil, false},
		"names only":          {nil, []string{"web"}, []string{"web"}, false},
		"neither shows every": {nil, nil, nil, true},
	} {
		t.Run(name, func(t *testing.T) {
			got := ObtainedContext(tc.paths, tc.named, owning)
			if got.Kind != ContextObtained || !reflect.DeepEqual(got.Paths, tc.paths) {
				t.Errorf("activity = %+v", got)
			}
			if len(got.Zones) != len(tc.want) || (len(tc.want) > 0 && !reflect.DeepEqual(got.Zones, tc.want)) {
				t.Errorf("Zones = %v, want %v", got.Zones, tc.want)
			}
			if got.showsEveryZone() != tc.everyZone {
				t.Errorf("showsEveryZone = %v, want %v", got.showsEveryZone(), tc.everyZone)
			}
		})
	}
}

func TestObtainedContextLeavesTheNamedZonesAlone(t *testing.T) {
	named := []string{"web", "domain"}
	ObtainedContext([]string{"a.go"}, named, func(string) []string { return []string{"app"} })
	if !reflect.DeepEqual(named, []string{"web", "domain"}) {
		t.Errorf("named = %v; ObtainedContext sorted its caller's slice", named)
	}
}

func TestChangedFileBearsDomainMeaningOnlyInTheModelsCode(t *testing.T) {
	inZones := ModelCode{Zones: []string{"ordering", "catalog"}}
	anywhere := ModelCode{Anywhere: true}
	for name, tc := range map[string]struct {
		owners []string
		test   bool
		model  ModelCode
		want   bool
	}{
		"in a context's Zone":                   {[]string{"domain", "ordering"}, false, inZones, true},
		"outside every context's Zone":          {[]string{"composition"}, false, inZones, false},
		"owned by no Zone":                      {nil, false, inZones, false},
		"a test in a context's Zone":            {[]string{"ordering"}, true, inZones, false},
		"anywhere while a context lacks a Zone": {[]string{"composition"}, false, anywhere, true},
		"anywhere, owned by no Zone":            {nil, false, anywhere, true},
		"a test while anywhere":                 {nil, true, anywhere, false},
	} {
		t.Run(name, func(t *testing.T) {
			got := ChangedFile("internal/x.go", tc.owners, tc.test, tc.model)
			if got.Kind != FilesChanged || !reflect.DeepEqual(got.Paths, []string{"internal/x.go"}) || !reflect.DeepEqual(got.Zones, tc.owners) {
				t.Errorf("activity = %+v", got)
			}
			if got.BearsDomain != tc.want {
				t.Errorf("BearsDomain = %v, want %v", got.BearsDomain, tc.want)
			}
		})
	}
}
