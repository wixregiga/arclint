package cmlvocab

import (
	"os"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

func syncFixtureRepository(t *testing.T, content string) Repository {
	t.Helper()
	rec, _ := vocab.NewDomainRecording("domain.arclint.cml")
	r, err := NewRepository(t.TempDir(), rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.Path(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return r
}

func readSyncLanguage(t *testing.T, r Repository) vocab.UbiquitousLanguage {
	t.Helper()
	lang, _, err := r.RecordedLanguage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return lang
}

func readSyncFile(t *testing.T, r Repository) string {
	t.Helper()
	b, _ := os.ReadFile(r.Path())
	return string(b)
}

func recordSyncLanguage(t *testing.T, r Repository, lang vocab.UbiquitousLanguage) error {
	t.Helper()
	err := r.Record(lang)
	if err == nil && !sameLanguage(lang, readSyncLanguage(t, r)) {
		t.Fatal("Record changed language")
	}
	if err == nil {
		validateContextMapper(t, []byte(readSyncFile(t, r)))
	}
	return err
}

const authoredSyncModel = `// header comment line one
// header comment line two

Domain Shop {
  domainVisionStatement = "A shop."
}

ContextMap {
  contains sales, billing
  sales [U]->[D,CF] billing
}

BoundedContext sales {
  domainVisionStatement = "Selling."
  Aggregate Order {
    "An order."
    Entity Order {
      aggregateRoot
      OrderId id key
      String customerName
      def void confirm();
      "An order has lines."
      def void ensureHasLines();
      Repository OrderRepository {
        @Order find(@OrderId id);
      }
    }
    "A line."
    Entity OrderLine {
      LineId id key
      int quantity
    }
    "Raised when an order is placed."
    DomainEvent OrderPlaced
    "Pricing inside the aggregate."
    ValueObject Price {
      BigDecimal amount
    }
    "Does quotes."
    Service QuoteService {
      void quote();
    }
  }
  Module sales {
    "An order identity."
    ValueObject OrderId
  }
}

BoundedContext billing {
  domainVisionStatement = "Billing."
}
`

func TestRecordKeepsHeaderComments(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	lang.Description = "A changed shop."
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	f := readSyncFile(t, r)
	if strings.Count(f, "header comment line one") != 1 {
		t.Errorf("header duplicated or lost: %d", strings.Count(f, "header comment line one"))
	}
}

func TestRecordMovesSurvivorsOfRemovedAggregate(t *testing.T) {
	source := strings.Replace(authoredSyncModel, "    \"Does quotes.\"", `    // Eligibility policy.
    "Can be shipped." ValueObject Shippable { def boolean satisfiedBy(); int attempts }
    "Does quotes."`, 1)
	r := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, r)
	t.Logf("old: %#v", normalized(lang).Contexts[0])
	c := &lang.Contexts[0]
	c.Aggregates = nil
	for i := range c.Events {
		c.Events[i].RaisedBy = ""
	}
	if _, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, lang.Relations); err != nil {
		t.Fatalf("want invalid: %v", err)
	}
	err := recordSyncLanguage(t, r, lang)
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"BigDecimal amount", "quote();", "Eligibility policy.", "int attempts"} {
		if !strings.Contains(readSyncFile(t, r), content) {
			t.Fatalf("surviving foreign content lost: %s", content)
		}
	}
}

func TestRecordReordersValuesAcrossContainers(t *testing.T) {
	src := strings.Replace(authoredSyncModel, "    \"An order identity.\"\n    ValueObject OrderId\n", "    \"An order identity.\"\n    ValueObject OrderId\n    \"Money.\"\n    ValueObject Money\n", 1)
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	t.Logf("values before: %v", c.ValueObjects)
	// Reverse order of value objects.
	for i, j := 0, len(c.ValueObjects)-1; i < j; i, j = i+1, j-1 {
		c.ValueObjects[i], c.ValueObjects[j] = c.ValueObjects[j], c.ValueObjects[i]
	}
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Errorf("S3 reorder refused: %v", err)
	}
}

func TestRecordReordersRootInvariants(t *testing.T) {
	src := strings.Replace(authoredSyncModel, "      def void ensureHasLines();\n", "      def void ensureHasLines();\n      \"An order is priced.\"\n      def void ensurePriced();\n", 1)
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	a := &lang.Contexts[0].Aggregates[0]
	a.Invariants[0], a.Invariants[1] = a.Invariants[1], a.Invariants[0]
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Errorf("S3b reorder refused: %v", err)
	}
}

func TestRecordAddsAggregateAfterModuleAggregate(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap { contains sales }
BoundedContext sales {
  domainVisionStatement = "Selling."
  Module Core {
    Aggregate Order {
      "An order."
      Entity Order {
        aggregateRoot
        OrderId id key
      }
    }
  }
}
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	c.Aggregates = append(c.Aggregates, vocab.Aggregate{Name: "Invoice", Definition: "An invoice.", Identity: "InvoiceId"})
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Errorf("S4 refused: %v", err)
	}
	got := readSyncLanguage(t, r)
	t.Logf("order after: %s, %s", got.Contexts[0].Aggregates[0].Name, got.Contexts[0].Aggregates[1].Name)
}

func TestRecordKeepsDirectiveOutsideContextBody(t *testing.T) {
	src := strings.Replace(authoredSyncModel, "BoundedContext billing {", "// @arclint question \"who-bills\" \"Who bills?\"\nBoundedContext billing {", 1)
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	t.Logf("questions read on billing: %v", lang.Contexts[1].Questions)
	lang.Contexts[1].Definition = "Billing changed."
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readSyncFile(t, r), "who-bills") {
		t.Errorf("S5: directive outside braces silently deleted")
	}
}

func TestRecordAvoidsExistingHandleCollision(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap { contains sales }
BoundedContext sales {
  domainVisionStatement = "Selling."
  Aggregate Order {
    "An order."
    // @arclint name "Order Line"
    Entity Order_Line {
      aggregateRoot
      LineId id key
    }
  }
}
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	c.ValueObjects = append(c.ValueObjects, vocab.ValueObject{Name: "Order_Line", Definition: "A value spelled like the handle."})
	err := recordSyncLanguage(t, r, lang)
	f := readSyncFile(t, r)
	t.Logf("err=%v declarations named Order_Line: %d", err, strings.Count(f, " Order_Line"))
	if err == nil && strings.Count(f, "Entity Order_Line")+strings.Count(f, "ValueObject Order_Line") == 2 {
		t.Errorf("S6: two declarations share the CML name Order_Line in one context")
	}
}

func TestRecordRemovesOptionalCrossReferences(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap {
  contains sales, billing, crm
  sales [U]->[D,CF] billing {
    exposedAggregates = Order
  }
  billing [U]->[D,CF] crm
}
BoundedContext sales {
  domainVisionStatement = "Selling."
  Aggregate Order {
    "An order."
    Entity Order {
      aggregateRoot
      OrderId id key
    }
  }
  Aggregate Cart {
    "A cart."
    Entity Cart {
      aggregateRoot
      CartId id key
    }
  }
}
BoundedContext billing {
  domainVisionStatement = "Billing."
}
BoundedContext crm realizes billing {
  type = TEAM
  domainVisionStatement = "CRM."
}
Stakeholders of billing {
  Stakeholder Accountant
}
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	// Remove aggregate Order (named by exposedAggregates).
	lang.Contexts[0].Aggregates = lang.Contexts[0].Aggregates[1:]
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	f := readSyncFile(t, r)
	if strings.Contains(f, "exposedAggregates = Order") {
		t.Errorf("S7a: exposedAggregates still names removed aggregate Order")
	}
	// Remove context billing.
	lang = readSyncLanguage(t, r)
	var keep []vocab.BoundedContext
	for _, c := range lang.Contexts {
		if c.Name != "billing" {
			keep = append(keep, c)
		}
	}
	lang.Contexts = keep
	lang.Relations = nil
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	f = readSyncFile(t, r)
	for _, s := range []string{"realizes billing", "Stakeholders of billing"} {
		if strings.Contains(f, s) {
			t.Errorf("S7b: dangling reference %q to removed context", s)
		}
	}
}

func TestReadNameDirectivesAfterClosingBrace(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap { contains sales, Billing_Dept }
BoundedContext sales {
  domainVisionStatement = "Selling."
}
// @arclint name "billing-dept"
BoundedContext Billing_Dept {
  domainVisionStatement = "Billing."
  Aggregate Inv {
    // @arclint name "Invoice Doc"
    "An invoice."
    Entity Inv {
      aggregateRoot
      InvId id key
    }
  }
}
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	t.Logf("contexts: %q %q aggregate: %q", lang.Contexts[0].Name, lang.Contexts[1].Name, lang.Contexts[1].Aggregates[0].Name)
	if lang.Contexts[1].Name != "billing-dept" {
		t.Errorf("S9a: name directive after closing brace not honored: %q", lang.Contexts[1].Name)
	}
	if lang.Contexts[1].Aggregates[0].Name != "Invoice Doc" {
		t.Errorf("S9b: name directive above doc string not honored: %q", lang.Contexts[1].Aggregates[0].Name)
	}
}

func TestRecordRenamesImplementedDomainReference(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap { contains sales }
BoundedContext sales implements Shop {
  domainVisionStatement = "Selling."
}
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	lang.Project = "Store"
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	f := readSyncFile(t, r)
	if strings.Contains(f, "implements Shop") && !strings.Contains(f, "Domain Shop") {
		t.Errorf("S10: implements still names the old Domain name")
	}
}

func TestRecordAliasEscapes(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	lang.Contexts[0].Aggregates[0].Aliases = []string{"100%2C off", "a,b", "x=y", "%25"}
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	got := readSyncLanguage(t, r)
	t.Logf("aliases: %q", got.Contexts[0].Aggregates[0].Aliases)
}

func TestRecordChangesRelationKindPreservingExtras(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap {
  contains sales, billing
  // ordinary comment above
  sales [U,OHS]->[D,ACL] billing : SalesBilling {
    implementationTechnology = "REST"
    exposedAggregates = Order
    downstreamRights = VETO_RIGHT
  }
}
BoundedContext sales {
  domainVisionStatement = "Selling."
  Aggregate Order {
    "An order."
    Entity Order {
      aggregateRoot
      OrderId id key
    }
  }
}
BoundedContext billing {
  domainVisionStatement = "Billing."
}
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	t.Logf("kind read: %s", lang.Relations[0].Kind)
	lang.Relations[0].Kind = vocab.RelationCustomerSupplier
	lang.Relations[0].Description = "desc"
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	lang = readSyncLanguage(t, r)
	lang.Relations[0].Kind = vocab.RelationPartnership
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	lang = readSyncLanguage(t, r)
	lang.Relations[0].Kind = vocab.RelationSeparateWays
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	lang = readSyncLanguage(t, r)
	lang.Relations[0].Kind = vocab.RelationConformist
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
}

func TestRecordMovesEventBetweenAggregates(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	c.Aggregates = append(c.Aggregates, vocab.Aggregate{Name: "Cart", Definition: "A cart.", Identity: "CartId"})
	c.Events[0].RaisedBy = "Cart"
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	lang = readSyncLanguage(t, r)
	lang.Contexts[0].Events[0].RaisedBy = "Order"
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
}

func TestRecordNoopPreservesCRLF(t *testing.T) {
	src := strings.ReplaceAll(authoredSyncModel, "\n", "\r\n")
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	if readSyncFile(t, r) != src {
		t.Errorf("S15: bytes changed")
	}
}

func TestRecordReusesEntityInNewAggregate(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	line := c.Aggregates[0].Entities[0]
	c.Aggregates[0].Entities = nil
	c.Aggregates = append(c.Aggregates, vocab.Aggregate{Name: "Cart", Definition: "A cart.", Identity: "CartId", Entities: []vocab.Entity{line}})
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readSyncFile(t, r), "int quantity") {
		t.Errorf("S16: foreign attribute lost on entity move")
	}
}

func TestRecordConvertsValueToSpecification(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	var vals []vocab.ValueObject
	for _, v := range c.ValueObjects {
		if v.Name == "Price" {
			c.Specifications = append(c.Specifications, vocab.Specification{Name: "Price", Definition: v.Definition})
		} else {
			vals = append(vals, v)
		}
	}
	c.ValueObjects = vals
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readSyncFile(t, r), "BigDecimal amount") {
		t.Errorf("S17: foreign attribute lost")
	}
}

func TestRecordKeepsTeamRelation(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap {
  contains sales, billing, Team1
  sales [U]->[D,CF] billing
  Team1 [P]<->[P] sales
}
BoundedContext sales { domainVisionStatement = "Selling." }
BoundedContext billing { domainVisionStatement = "Billing." }
BoundedContext Team1 { type = TEAM }
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	lang.Relations = nil
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	f := readSyncFile(t, r)
	if !strings.Contains(f, "Team1 [P]<->[P] sales") || !strings.Contains(f, "Team1") {
		t.Errorf("S18: team relationship or contains lost")
	}
}

func TestRecordRemovesTeamRelationToRemovedContext(t *testing.T) {
	src := `Domain Shop { domainVisionStatement = "A shop." }
ContextMap {
  contains sales, billing, Team1
  sales [U]->[D,CF] billing
  Team1 [P]<->[P] billing
}
BoundedContext sales { domainVisionStatement = "Selling." }
BoundedContext billing { domainVisionStatement = "Billing." }
BoundedContext Team1 { type = TEAM }
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	lang.Contexts = lang.Contexts[:1]
	lang.Relations = nil
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	f := readSyncFile(t, r)
	if strings.Contains(f, "Team1 [P] <-> [P] billing") {
		t.Errorf("S19: relationship still names removed context billing")
	}
}

func TestRecordInvariantDigitSegments(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	a := &lang.Contexts[0].Aggregates[0]
	a.Invariants = append(a.Invariants, vocab.Invariant{Key: "tier-2-pricing", Statement: "Tier two."}, vocab.Invariant{Key: "x-1", Statement: "x one."}, vocab.Invariant{Key: "x1", Statement: "x1."})
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	got := readSyncLanguage(t, r)
	t.Logf("%v", got.Contexts[0].Aggregates[0].Invariants)
}

func TestRecordSeparateWaysToNativeRelationDescription(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"same description", "desc", "desc"},
		{"different description", "desc", "other"},
		{"empty description", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "Domain Shop { domainVisionStatement = \"A shop.\" }\nContextMap {\n  contains sales, billing\n  // @arclint separate-ways \"sales\" \"billing\" \"" + tc.before + "\"\n}\nBoundedContext sales { domainVisionStatement = \"Selling.\" }\nBoundedContext billing { domainVisionStatement = \"Billing.\" }\n"
			r := syncFixtureRepository(t, src)
			lang := readSyncLanguage(t, r)
			lang.Relations[0].Kind = vocab.RelationConformist
			lang.Relations[0].Description = tc.after
			if err := recordSyncLanguage(t, r, lang); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
}

func TestRecordRemovesAggregateAndItsTenants(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	c.Aggregates = nil
	c.Events = nil
	c.Services = nil
	c.ValueObjects = c.ValueObjects[1:] // drop Price, keep OrderId
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Errorf("refused: %v", err)
	}
}

func TestRecordRenamesMapProjectThenDescribesIt(t *testing.T) {
	src := `ContextMap Shop {
  contains sales
}
BoundedContext sales { domainVisionStatement = "Selling." }
`
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	t.Logf("project=%q", lang.Project)
	lang.Project = "Store"
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	lang = readSyncLanguage(t, r)
	lang.Description = "A store."
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	if strings.Index(readSyncFile(t, r), "Domain Store") > strings.Index(readSyncFile(t, r), "ContextMap") {
		t.Fatal("Domain must precede ContextMap")
	}
	lang = readSyncLanguage(t, r)
	if lang.Project != "Store" || lang.Description != "A store." {
		t.Errorf("got %q %q", lang.Project, lang.Description)
	}
}

func TestRecordAddsDomainBeforeNamedMap(t *testing.T) {
	src := "ContextMap Shop {\n  contains sales\n}\nBoundedContext sales { domainVisionStatement = \"Selling.\" }\n"
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	lang.Description = "A shop."
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Errorf("refused: %v", err)
	}
	output := readSyncFile(t, r)
	if strings.Index(output, "Domain Shop") > strings.Index(output, "ContextMap") {
		t.Fatal("Domain must precede ContextMap")
	}
	file, err := syntax.Parse([]byte(output))
	if err != nil {
		t.Fatal(err)
	}
	if file.Map.Name != "" || file.Map.Present["name"] {
		t.Fatal("map retained its old name or presence marker")
	}
}

func TestRecordAddsDescriptionAfterMapProjectRename(t *testing.T) {
	src := "ContextMap Shop {\n  contains sales\n}\nBoundedContext sales { domainVisionStatement = \"Selling.\" }\n"
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	lang.Project = "Store"
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	lang = readSyncLanguage(t, r)
	lang.Description = "A store."
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Errorf("refused: %v", err)
	}
}

func TestRecordExportsIntoDifferentlyOrderedTarget(t *testing.T) {
	r := syncFixtureRepository(t, authoredSyncModel)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	c.ValueObjects[0], c.ValueObjects[1] = c.ValueObjects[1], c.ValueObjects[0]
	err := r.Record(lang)
	if err != nil {
		t.Fatal(err)
	}
	validateContextMapper(t, []byte(readSyncFile(t, r)))
	if !sameLanguage(lang, readSyncLanguage(t, r)) {
		t.Fatal("export changed language")
	}
}

func TestRecordRemovesAggregateAndBoundEvents(t *testing.T) {
	src := strings.Replace(authoredSyncModel, "    \"An order identity.\"\n", "    \"Shipped.\"\n    DomainEvent OrderShipped {\n      belongsTo @Order\n    }\n    \"An order identity.\"\n", 1)
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	c := &lang.Contexts[0]
	c.Aggregates = nil
	c.Events = nil
	c.Services = nil
	c.ValueObjects = c.ValueObjects[1:]
	err := recordSyncLanguage(t, r, lang)
	t.Logf("err: %v", err)
}

func TestRecordAddsGloballyUniqueAggregateContainers(t *testing.T) {
	lang, err := vocab.NewUbiquitousLanguage("Shop", "", []vocab.BoundedContext{
		{Name: "sales", Definition: "Selling.", Aggregates: []vocab.Aggregate{{Name: "Order", Definition: "An order.", Identity: "OrderId"}}},
		{Name: "billing", Definition: "Billing.", Aggregates: []vocab.Aggregate{{Name: "Order", Definition: "A billed order.", Identity: "OrderId"}}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := syncFixtureRepository(t, "")
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	lang = readSyncLanguage(t, r)
	lang.Contexts[0].Aggregates = append(lang.Contexts[0].Aggregates, vocab.Aggregate{Name: "Cart", Definition: "A cart.", Identity: "CartId"})
	lang.Contexts[1].Aggregates = append(lang.Contexts[1].Aggregates, vocab.Aggregate{Name: "Cart", Definition: "A billed cart.", Identity: "CartId"})
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	f := readSyncFile(t, r)
	t.Logf("Aggregate names: %v", func() []string {
		var out []string
		for _, l := range strings.Split(f, "\n") {
			if strings.Contains(l, "Aggregate ") {
				out = append(out, strings.TrimSpace(l))
			}
		}
		return out
	}())
}

func TestRecordKeepsForeignHintSpacingEntries(t *testing.T) {
	src := strings.Replace(authoredSyncModel, "      aggregateRoot\n      OrderId id key\n", "      aggregateRoot\n      hint = \"umlStereotype=root, arclint.alias=purchase, color=red\"\n      OrderId id key\n", 1)
	r := syncFixtureRepository(t, src)
	lang := readSyncLanguage(t, r)
	t.Logf("aliases read: %q", lang.Contexts[0].Aggregates[0].Aliases)
	lang.Contexts[0].Aggregates[0].Aliases = []string{"sale"}
	if err := recordSyncLanguage(t, r, lang); err != nil {
		t.Fatal(err)
	}
	got := readSyncLanguage(t, r)
	t.Logf("aliases after: %q", got.Contexts[0].Aggregates[0].Aliases)
	if !strings.Contains(readSyncFile(t, r), "umlStereotype=root") || !strings.Contains(readSyncFile(t, r), "color=red") {
		t.Errorf("foreign hint entries lost")
	}
}

func TestRecordReordersRelationsAndContextsAroundForeignSlots(t *testing.T) {
	source := `Domain Shop
ContextMap {
 contains sales, Team1, billing, crm
 // Sales to billing.
 sales [U]->[D,CF] billing
 // Team collaboration.
 Team1 [P]<->[P] sales
 // Billing to CRM.
 billing [U]->[D,CF] crm
}
BoundedContext sales { domainVisionStatement "Selling." }
BoundedContext Team1 { type TEAM }
BoundedContext billing { domainVisionStatement "Billing." }
BoundedContext crm { domainVisionStatement "CRM." }
`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Relations[0], lang.Relations[1] = lang.Relations[1], lang.Relations[0]
	lang.Contexts[0], lang.Contexts[2] = lang.Contexts[2], lang.Contexts[0]
	assertRecorded(t, repo, lang)
	file, err := syntax.Parse([]byte(readSyncFile(t, repo)))
	if err != nil {
		t.Fatal(err)
	}
	if file.Map.BoundedContexts[1].Name != "Team1" || file.BoundedContexts[1].Name != "Team1" {
		t.Fatal("foreign context slot moved")
	}
	from, _, _, _ := relationshipValue(file.Map.Relationships[1])
	if from != "Team1" {
		t.Fatal("foreign relationship slot moved")
	}
	_, _, _, first := relationshipValue(file.Map.Relationships[0])
	_, _, _, last := relationshipValue(file.Map.Relationships[2])
	if len(first.LeadingComments) != 1 || first.LeadingComments[0].Text != "// Billing to CRM." || len(last.LeadingComments) != 1 || last.LeadingComments[0].Text != "// Sales to billing." {
		t.Fatal("relationship leading comments did not move with declarations")
	}
}

func TestRecordReordersQuestionCommentsWithTheirDirective(t *testing.T) {
	source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 // First question note.
 // @arclint question "first" "First?"
 // Second question note.
 // @arclint question "second" "Second?"
 // Context end note.
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts[0].Questions[0], lang.Contexts[0].Questions[1] = lang.Contexts[0].Questions[1], lang.Contexts[0].Questions[0]
	assertRecorded(t, repo, lang)
	output := readSyncFile(t, repo)
	if !strings.Contains(output, "// Second question note.\n  // @arclint question \"second\" \"Second?\"") || !strings.Contains(output, "// First question note.\n  // @arclint question \"first\" \"First?\"") {
		t.Fatal("leading comments did not travel with question directives")
	}
	if strings.Index(output, "Context end note.") < strings.Index(output, "First?") {
		t.Fatal("foreign trailing note moved before questions")
	}
}

func TestRecordQuestionOrderKeepsGrammarDefinitionComments(t *testing.T) {
	source := `Domain Shop
BoundedContext sales {
 domainVisionStatement "Selling."
 Module sales {
  /* Money is an amount. */
  // @arclint question "rounding" "How do we round?"
  ValueObject Money
  /* Addresses locate customers. */
  // @arclint question "display" "How do we display?"
  ValueObject Address
 }
}`
	repo := syncFixtureRepository(t, source)
	lang := readSyncLanguage(t, repo)
	lang.Contexts[0].Questions[0], lang.Contexts[0].Questions[1] = lang.Contexts[0].Questions[1], lang.Contexts[0].Questions[0]
	assertRecorded(t, repo, lang)
	output := readSyncFile(t, repo)
	for _, comment := range []string{"/* Money is an amount. */", "/* Addresses locate customers. */"} {
		if strings.Count(output, comment) != 1 {
			t.Fatalf("definition comment lost or duplicated: %s", comment)
		}
	}
}
