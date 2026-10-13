package cmlvocab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func shuffleSection[T any](random *rand.Rand, section []T) {
	random.Shuffle(len(section), func(i, j int) { section[i], section[j] = section[j], section[i] })
}

func permuteLanguage(random *rand.Rand, lang *vocab.UbiquitousLanguage) {
	shuffleSection(random, lang.Contexts)
	shuffleSection(random, lang.Relations)
	for i := range lang.Contexts {
		c := &lang.Contexts[i]
		shuffleSection(random, c.Aggregates)
		shuffleSection(random, c.ValueObjects)
		shuffleSection(random, c.Specifications)
		shuffleSection(random, c.Events)
		shuffleSection(random, c.Services)
		shuffleSection(random, c.Questions)
		for j := range c.Aggregates {
			a := &c.Aggregates[j]
			shuffleSection(random, a.Entities)
			shuffleSection(random, a.Aliases)
			shuffleSection(random, a.Invariants)
			shuffleSection(random, a.Assertions)
			for k := range a.Entities {
				shuffleSection(random, a.Entities[k].Aliases)
			}
		}
		for j := range c.ValueObjects {
			v := &c.ValueObjects[j]
			shuffleSection(random, v.Aliases)
			shuffleSection(random, v.Invariants)
		}
		for j := range c.Services {
			shuffleSection(random, c.Services[j].Assertions)
		}
	}
}

// A declaration may move to satisfy order. Compare its foreign body independently
// of that modeled placement, while retaining order inside its attributes,
// parameters, ordinary operations, foreign hints and comment runs.
func foreignBodies(t *testing.T, data []byte) []byte {
	t.Helper()
	var value any
	if err := json.Unmarshal(foreignProjectionWithModules(t, data, true), &value); err != nil {
		t.Fatal(err)
	}
	declarationFields := map[string]bool{"Domains": true, "BoundedContexts": true, "Aggregates": true, "Modules": true, "DomainObjects": true, "DomainServices": true, "Services": true, "Relationships": true}
	var bodies []string
	var clean func(any) any
	var collect func(any)
	collect = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			body, err := json.Marshal(clean(v))
			if err != nil {
				t.Fatal(err)
			}
			bodies = append(bodies, string(body))
		case []any:
			for _, child := range v {
				collect(child)
			}
		default:
			return
		}
	}
	clean = func(value any) any {
		switch v := value.(type) {
		case map[string]any:
			// An explicit belongsTo added to preserve a moved event's RaisedBy
			// requires braces. Those braces are part of the modeled carrier,
			// not an authored event body, when no foreign body exists.
			if _, event := v["Persistent"]; event && v["Attributes"] == nil && v["References"] == nil && v["Operations"] == nil && v["Associations"] == nil && v["Repository"] == nil && v["Hint"] == "" && v["TrailingComments"] == nil {
				v["ExplicitBody"] = false
			}
			for field, child := range v {
				if declarationFields[field] {
					collect(child)
					delete(v, field)
				} else {
					v[field] = clean(child)
				}
			}
			return v
		case []any:
			for i, child := range v {
				v[i] = clean(child)
			}
			return v
		default:
			return value
		}
	}
	collect(value)
	sort.Strings(bodies)
	body, err := json.Marshal(bodies)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestRecordPermutationsOfRealCorpus(t *testing.T) {
	t.Parallel()
	iterations := 16
	if !exhaustive() {
		iterations = 1
	}
	paths := []string{"insurance-example/Insurance-Example-Stage-5.cml", "ddd-sample/DDD-Sample-Stage-5.cml", "same-day-delivery-shop/stakeholders-and-values.cml", "workflow-samples/InsuranceQuoteRequest.cml"}
	random := rand.New(rand.NewPCG(2026, 1012))
	for _, path := range sampledModels(paths) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			repo := syncFixtureRepository(t, string(projectableCorpus(t, path)))
			base := addSyncElements(t, readSyncLanguage(t, repo))
			assertRecorded(t, repo, base)
			before := foreignBodies(t, []byte(readSyncFile(t, repo)))
			for iteration := 0; iteration < iterations; iteration++ {
				lang := readSyncLanguage(t, repo)
				permuteLanguage(random, &lang)
				t.Run(fmt.Sprint(iteration), func(t *testing.T) {
					assertRecorded(t, repo, lang)
					after := foreignBodies(t, []byte(readSyncFile(t, repo)))
					if !bytes.Equal(before, after) {
						t.Fatal(foreignBodyDifference(before, after))
					}
				})
			}
		})
	}
}

func TestRecordPermutesEveryOrderedSection(t *testing.T) {
	t.Parallel()
	// Fresh recording supplies several values in every section; authored attributes
	// and comments make their preservation independently observable.
	baseRepo := syncFixtureRepository(t, authoredSyncModel)
	lang := addSyncElements(t, readSyncLanguage(t, baseRepo))
	c := &lang.Contexts[0]
	c.Aggregates[0].Invariants = append(c.Aggregates[0].Invariants, vocab.Invariant{Key: "priced", Statement: "The order is priced."})
	c.Aggregates[0].Assertions = []vocab.Assertion{{Key: "confirmed", On: "confirm", Statement: "Confirmation sets status."}, {Key: "cancelled", On: "cancel", Statement: "Cancellation clears status."}}
	c.Aggregates[0].Aliases = []string{"purchase", "sale"}
	c.ValueObjects[0].Aliases = []string{"charge", "cost"}
	c.Aggregates[0].Entities[0].Aliases = []string{"line", "item"}
	c.Aggregates[0].Entities = append(c.Aggregates[0].Entities, vocab.Entity{Name: "Delivery", Definition: "A delivery."})
	c.ValueObjects[0].Invariants = []vocab.Invariant{{Key: "positive", Statement: "A positive amount."}, {Key: "finite", Statement: "A finite amount."}}
	c.Specifications = append(c.Specifications, vocab.Specification{Name: "Available", Definition: "Available stock."})
	c.Questions = append(c.Questions, vocab.Question{Key: "who-sells", Text: "Who sells?"})
	c.Services[0].Assertions = []vocab.Assertion{{Key: "quoted", On: "quote", Statement: "Quotes a price."}, {Key: "expired", On: "expire", Statement: "Expires the quote."}}
	lang.Contexts = append(lang.Contexts, vocab.BoundedContext{Name: "shipping", Definition: "Shipping."})
	lang.Relations = append(lang.Relations, vocab.ContextRelation{From: "sales", To: "shipping", Kind: vocab.RelationSeparateWays, Description: "Independent."}, vocab.ContextRelation{From: "billing", To: "shipping", Kind: vocab.RelationPartnership, Description: "Partners."})
	data, err := Render(lang)
	if err != nil {
		t.Fatal(err)
	}
	repo := syncFixtureRepository(t, string(data))
	random := rand.New(rand.NewPCG(7, 31))
	permutations := 32
	if !exhaustive() {
		permutations = 3
	}
	for i := 0; i < permutations; i++ {
		lang = readSyncLanguage(t, repo)
		permuteLanguage(random, &lang)
		assertRecorded(t, repo, lang)
	}
}

func foreignBodyDifference(before, after []byte) string {
	var a, b []string
	_ = json.Unmarshal(before, &a)
	_ = json.Unmarshal(after, &b)
	remaining := map[string]int{}
	for _, body := range b {
		remaining[body]++
	}
	for _, body := range a {
		if remaining[body] > 0 {
			remaining[body]--
			continue
		}
		var old map[string]any
		_ = json.Unmarshal([]byte(body), &old)
		for _, candidate := range b {
			if remaining[candidate] == 0 {
				continue
			}
			var next map[string]any
			_ = json.Unmarshal([]byte(candidate), &next)
			if old["Name"] != next["Name"] {
				continue
			}
			var fields []string
			for key, value := range old {
				if !reflect.DeepEqual(value, next[key]) {
					fields = append(fields, key)
				}
			}
			return fmt.Sprintf("foreign body %v changed fields %v", old["Name"], fields)
		}
		return fmt.Sprintf("foreign body %v disappeared", old["Name"])
	}
	return "additional foreign body appeared"
}
