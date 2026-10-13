package syntax

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func parseRule(t *testing.T, rule, source string) any {
	t.Helper()
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	ts, err := lex(rule, []byte(source), g)
	if err != nil {
		t.Fatal(err)
	}
	p := parser{grammar: g, tokens: ts, expected: map[string]bool{}, file: rule}
	var result any
	if !p.rule(rule, 0, func(end int, d *derivation) bool {
		if end != len(ts)-1 {
			p.expect(end, "end of fragment")
			return false
		}
		result = d.value
		return true
	}) {
		t.Fatalf("%s: cannot parse %q, failure token %d, expected %v, internal %v", rule, source, p.farthest, p.expected, p.internal)
	}
	return result
}

func TestEveryGrammarRule(t *testing.T) {
	// These inputs exercise the public typed AST for every concrete, union,
	// datatype and enum production, including inherited tactic-only entry rules.
	cases := map[string]string{
		"ContextMappingModel": "", "Import": `import 'x.cml'`, "ContextMap": `ContextMap MapName {state AS_IS type ORGANIZATIONAL contains A,B A -> B}`,
		"BoundedContext": `BoundedContext A realizes B implements ^D refines ^C { type TEAM domainVisionStatement "vision" responsibilities "one","two" implementationTechnology "Go" knowledgeLevel CONCRETE businessModel "business" evolution PRODUCT Application App Aggregate G Module M Service Svc }`,
		"DomainPart":     `Domain ^D`, "Domain": `Domain ^D { Subdomain ^S domainVisionStatement "v" }`, "Subdomain": `Subdomain ^S supports ^U {domainVisionStatement "v" type CORE_DOMAIN Entity E Service Svc}`,
		"Relationship": `A -> B`, "SymmetricRelationship": `A Partnership B`, "Partnership": `[P] A <-> B [P] : R {implementationTechnology "REST"}`, "SharedKernel": `A Shared-Kernel B`,
		"UpstreamDownstreamRelationship": `A[PL] Upstream-Downstream [ACL]B {downstreamRights VETO_RIGHT exposedAggregates G,H implementationTechnology "REST"}`,
		"CustomerSupplierRelationship":   `A[U,S,OHS,PL] -> [D,C,ACL,CF] B`,
		"Aggregate":                      `"doc" Aggregate G { responsibilities "one","two" features ^U owner A knowledgeLevel CONCRETE structuralVolatility OFTEN contentVolatility RARELY availabilityCriticality HIGH consistencyCriticality LOW storageSimilarity HUGE securityCriticality NORMAL securityZone "internal" securityAccessGroup "staff" Entity E ValueObject V DomainEvent Ev Command Cmd Service Svc Resource R Consumer ^C {} }`,
		"Application":                    `Application App {Command ^C DomainEvent E Service Svc Flow F Coordination Co}`,
		"Coordination":                   `Coordination Co { A::Svc::op; }`, "CoordinationStep": `A::Svc::op;`, "Flow": `Flow F { command ^C emits event E event E triggers operation op }`, "FlowStep": `command ^C emits event E`,
		"DomainEventProductionStep": `operation op [initiated by "actor"] delegates to G [BEFORE,OTHER -> AFTER* X LAST] emits event E + F`,
		"CommandInvokationStep":     `event E + F triggers command ^C X ^D`, "EitherCommandOrOperation": `command ^C [initiated by "actor"]`, "EitherCommandOrOperationInvokation": `operation op + other`,
		"CommandInvokation": `command ^C O ^D`, "SingleCommandInvokation": `^C`, "ConcurrentCommandInvokation": `^C + ^D`, "ExclusiveAlternativeCommandInvokation": `^C x ^D`, "InclusiveAlternativeCommandInvokation": `^C o ^D`,
		"OperationInvokation": `operation op X other`, "SingleOperationInvokation": `op`, "ConcurrentOperationInvokation": `op + other`, "ExclusiveAlternativeOperationInvokation": `op x other`, "InclusiveAlternativeOperationInvokation": `op o other`,
		"EventProduction": `E X F`, "SingleEventProduction": `E`, "MultipleEventProduction": `E + F`, "ExclusiveAlternativeEventProduction": `E x F`, "InclusiveAlternativeEventProduction": `E o F`,
		"UserRequirement": `UseCase ^U`, "UseCase": `UseCase ^U {level "summary" scope "system" benefit "benefit" secondaryActors "other","another" actor "actor" interactions read an "item" with their "code","cost" in a "cart"}`,
		"UserStory": `UserStory ^U split by Other { As a "actor" I want to create an "item" so that "benefit" and that "privacy","trust" are promoted, accepting that "speed" is harmed }`,
		"Feature":   `read a "thing"`, "NormalFeature": `"find" the "item" with its "id" to an "account"`, "StoryFeature": `I want to delete "thing" for "owner"`, "StoryValuation": `and that "trust" is promoted accepting that "speed" is reduced`, "UserActivityDefaultVerb": `update`,
		"SculptorModule": `"doc" Module M {external basePackage = org.example hint = "h" Service Svc Resource R Consumer ^C {} Entity E Aggregate G}`,
		"Stakeholders":   `Stakeholders of A,B {Stakeholder Sth StakeholderGroup Gr}`, "AbstractStakeholder": `Stakeholder Sth`, "StakeholderGroup": `StakeholderGroup Gr { Stakeholder Sth }`, "Stakeholder": `Stakeholder Sth {description "d" interest HIGH influence MEDIUM}`,
		"ValueRegister": `ValueRegister V for A {ValueCluster ^C {core PRIVACY} Value Val ValueEpic Ep {As a Sth I value "v" as demonstrated in realization of "r" reduction of "h"} ValueNarrative N {When the SOI executes "f", stakeholders expect it to promote, protect or create "p", possibly degrading or prohibiting "h" with the following externally observable and/or internally auditable behavior: "b"} ValueWeigthing W {In the context of the SOI, stakeholder Sth values "a" more than "b" expecting benefits such as "benefit" running the risk of harms such as "harm"}}`,
		"ValueCluster":  `ValueCluster ^C { Value V core "custom" demonstrator "d" demonstrator "e" relatedValue "r" opposingValue "o" Stakeholder Sth }`, "Value": `Value V {isCore relatedValue "r" demonstrator "d" opposingValue "o" Stakeholders Gr {impact HIGH priority LOW consequences good "g" bad "b" neutral "n" action "a" MONITOR}}`,
		"ValueElicitation": `Stakeholder Sth {consequences good "g" action "a" "custom" impact MEDIUM priority HIGH}`, "ValueEpic": `ValueEpic E {As a Sth I value "v" as demonstrated in reduction of "h" reduction of "other" realization of "r"}`, "ValueNarrative": `ValueNarrative N {When the SOI executes "f", stakeholders expect it to promote, protect or create "p", possibly degrading or prohibiting "h" with the following externally observable and/or internally auditable behavior: "b"}`, "ValueWeigthing": `ValueWeigthing W {In the context of the SOI, stakeholder Sth values "a" more than "b" expecting benefits such as "benefit" running the risk of harms such as "harm"}`, "Consequence": `neutral "c" action "a" ACT`, "Action": `action "a" MONITOR`,
		"TacticDDDModel": `ApplicationPart App { Entity E }`, "TacticDDDApplication": `"doc" TacticDDDApplication App {basePackage = org.example Entity E}`,
		"Service":  `"doc" Service Svc {subscribe to orders/new eventBus = bus webservice hint = "h" nogap inject @Repo > helper void op() publish @Ev to orders => @Repo.save : write [ -> AFTER ]; -- "association" @E;}`,
		"Resource": `"doc" Resource R { path = "/r" scaffold hint = "h" gap inject @Svc @E get(@E arg) path = "/get" return = "body" GET hint = "h" throws org.Error => @Svc.op; }`,
		"Consumer": `"doc" Consumer ^C {hint = "h" inject @Svc unmarshall to @E subscribe to orders:new eventBus = bus queueName = channel.name}`, "Subscribe": `subscribe to ^a/b:c.d eventBus = bus`, "Publish": `publish @Ev to channel eventBus = bus`, "Event": `Event Ev`, "DomainObjectTypedElement": `String arg`,
		"ServiceOperation": `"doc" private List<@E> op("p" String arg) publish to topic hint = "h" throws org.Error,Other => Repo.save : read-only;`,
		"StateTransition":  `BEFORE,OTHER -> AFTER* x LAST`, "StateTransitionTarget": `AFTER* X LAST`, "SingleStateTransitionTarget": `AFTER*`, "ExclusiveAlternativeStateTransitionTarget": `AFTER X LAST`, "TargetState": `AFTER*`,
		"ServiceOperationDelegate": `=> @Repo.save`, "ServiceRepositoryOption": `Repository Repo`, "ServiceRepositoryOperationOption": `op();`, "ResourceOperation": `protected @E op() return = "r" path = "p" POST hint = "h" throws Error => Svc.op;`, "ResourceOperationDelegate": `=> @Svc.op`,
		"RepositoryOperation": `public Map<String,@E> find() build map construct orderBy = "o" groupBy = "g" select = "s" condition = "c" query = "q" nogap cache hint = "h" throws Error publish @Ev to topic => AccessObject;`, "Parameter": `"doc" Map<String,ListType<@E>> arg`, "ComplexType": `Map<@KeyType,Generic<@E>>`,
		"SimpleDomainObject": `Trait T`, "DomainObject": `ValueObject V`,
		"Entity":                `"doc" abstract Entity E extends @Base with @T with Other {package = org.example !auditable !optimisticLocking !cache nogap scaffold hint = "h" databaseTable = "t" discriminatorValue = "v" discriminatorColumn = "c" discriminatorType = CHAR discriminatorLength = "1" inheritanceType = JOINED validate = "v" aggregateRoot belongsTo @Owner String attr - Other ref def void op() : write [ -> AFTER ]; -- Other Repository Repo }`,
		"ValueObject":           `abstract ValueObject V extends org.Base with T {package = org.example !immutable !optimisticLocking !persistent cache gap scaffold hint = "h" databaseTable = "t" discriminatorValue = "v" discriminatorColumn = "c" discriminatorType = INTEGER discriminatorLength = "1" inheritanceType = SINGLE_TABLE validate = "v" aggregateRoot belongsTo Owner String attr - Other ref * op(); association "label" Other Repository Repo}`,
		"DomainEvent":           `abstract Event Ev extends @Base with @T {package = org.example persistent cache nogap scaffold hint = "h" databaseTable = "t" discriminatorValue = "v" discriminatorColumn = "c" discriminatorType = STRING discriminatorLength = "1" inheritanceType = JOINED validate = "v" aggregateRoot belongsTo Owner String attr reference Other ref def op(); -- Other Repository Repo}`,
		"CommandEvent":          `abstract CommandEvent Cmd extends org.Base with T {package = org.example persistent !cache gap scaffold hint = "h" databaseTable = "t" discriminatorValue = "v" discriminatorColumn = "c" discriminatorType = STRING discriminatorLength = "1" inheritanceType = JOINED validate = "v" aggregateRoot belongsTo Owner String attr - Other ref def op(); -- Other Repository Repo}`,
		"Trait":                 `Trait T {package = org.example hint = "h" String attr - Other ref def op(); -- Other}`,
		"DomainObjectOperation": `"doc" * abstract package Generic<@E> op(@E arg) hint = "h" throws Error : write [BEFORE -> AFTER];`,
		"DataTransferObject":    `"doc" abstract DataTransferObject Dto extends @Base {package = org.example nogap hint = "h" validate = "v" List<String> attr - List<@E> ref}`,
		"BasicType":             `"doc" BasicType B with T {package = org.example !immutable gap hint = "h" String attr - Other ref def op(); -- Other}`,
		"Attribute":             `"doc" public List<String> attr databaseType = "t" databaseColumn = "c" transient validate = "v" url = "u" scriptAssert = "s" length = "l" size = "s" range = "r" pattern = "p" notBlank = "b" notEmpty = "e" decimalMin = "m" decimalMax = "m" min = "m" max = "m" past = "p" future = "f" email = "e" digits = "d" creditCardNumber = "c" hint = "h" assertTrue = "t" assertFalse = "f" index nullable = "n" unique required !changeable key;`,
		"Reference":             `"doc" reference private List<@E> ref opposite other orderColumn = "oc" orderby = "o" transient validate = "v" valid = "v" size = "s" notEmpty = "e" databaseJoinColumn = "jc" databaseJoinTable = "jt" databaseColumn = "c" inverse !cache fetch = "f" cascade = "c" hint = "h" nullable = "n" required !changeable key -- "label";`,
		"Association":           `"doc" association "desc" @E;`,
		"DtoAttribute":          `"doc" protected List<String> attr key !changeable required nullable = "n" transient assertFalse = "f" assertTrue = "t" creditCardNumber = "c" digits = "d" email = "e" future = "f" past = "p" max = "m" min = "m" decimalMax = "m" decimalMin = "m" notEmpty = "e" notBlank = "b" pattern = "p" range = "r" size = "s" length = "l" scriptAssert = "s" url = "u" validate = "v" hint = "h";`,
		"DtoReference":          `"doc" - public List<@E> ref hint = "h" validate = "v" valid = "v" size = "s" notEmpty = "e" transient nullable = "n" !required !changeable key;`, "OppositeHolder": `opposite ref`,
		"Repository":        `"doc" Repository Repo {subscribe to topic hint = "h" nogap > helper inject @Svc op() => customAccess;}`,
		"ServiceDependency": `inject @Svc`, "Dependency": `> helper`,
		"Enum": `"doc" enum States {ordinal aggregateLifecycle hint = "h" package = org.example String label key; A("val",0), B;}`, "EnumAttribute": `"doc" String label key;`, "EnumValue": `"doc" A("v",0)`, "EnumParameter": `0`,
		"AnyProperty": `String attr`, "Property": `reference E ref`, "DtoProperty": `List<String> attr`, "Type": `Object[]`, "JavaIdentifier": `org.^package.^String`, "ChannelIdentifier": `^a.b/c:d`, "ThrowsIdentifier": `org.Error,Other`,
	}
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	for name, r := range g.rules {
		if r.enum {
			cases[name] = firstKeyword(r.body)
		}
	}
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			value := parseRule(t, name, cases[name])
			if value == nil {
				t.Fatal("missing AST value")
			}
			printer := printer{grammar: g}
			var tokens []printToken
			var ok bool
			if g.rules[name].enum || g.rules[name].datatype {
				tokens, ok = printer.scalar(name, value.(string))
			} else {
				tokens, ok = printer.node(name, value)
			}
			if !ok {
				t.Fatalf("cannot print %s from its typed AST", name)
			}
			output := layout(tokens)
			again := parseRule(t, name, string(output))
			if !reflect.DeepEqual(semantic(reflect.ValueOf(value)), semantic(reflect.ValueOf(again))) {
				t.Fatalf("%s semantic round trip changed: %s", name, output)
			}
		})
	}
	for name := range g.rules {
		if _, ok := cases[name]; !ok {
			t.Errorf("grammar rule %s has no exercised case", name)
		}
	}
}

func firstKeyword(e *expression) string {
	if e.kind == exprLiteral {
		return e.text
	}
	for _, c := range e.children {
		if s := firstKeyword(c); s != "" {
			return s
		}
	}
	return ""
}

func TestKeywordInventory(t *testing.T) {
	fresh := &grammar{rules: map[string]*grammarRule{}, keywords: map[string]bool{}}
	for _, path := range []string{"grammar/ContextMappingDSL.xtext", "grammar/TacticDDDLanguage.xtext"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = fresh.read(string(data)); err != nil {
			t.Fatal(err)
		}
	}
	table := map[string]bool{}
	for _, word := range reservedKeywords {
		if table[word] {
			t.Errorf("duplicate keyword %q", word)
		}
		table[word] = true
	}
	if !reflect.DeepEqual(table, fresh.keywords) {
		t.Fatal("Go keyword table drifted from pinned grammar parser and enum literals")
	}
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	for word := range table {
		if !IsKeyword(word) {
			t.Errorf("IsKeyword rejected lexer keyword %q", word)
		}
		ts, err := lex(word, []byte(word), g)
		if err != nil {
			t.Errorf("keyword %q: %s", word, err)
			continue
		}
		if len(ts) != 2 || ts[0].kind != word {
			t.Errorf("keyword %q became %v", word, ts)
		}
		if isIDStart(word[0]) && !strings.ContainsAny(word, " -[],/:<>") {
			ts, err := lex(word, []byte("^"+word), g)
			if err != nil || ts[0].kind != "ID" || ts[0].value != word {
				t.Errorf("escaped keyword %q did not become ID", word)
			}
		}
	}
}

func TestIsKeywordRejectsIdentifiers(t *testing.T) {
	for _, name := range []string{"", "Activity", "domain", "^Domain", "Domain1", "_Domain", "DomainRepository"} {
		if IsKeyword(name) {
			t.Errorf("identifier %q classified as keyword", name)
		}
	}
}

func TestIsKeywordAgreesWithLiteralTerminals(t *testing.T) {
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	for _, terminal := range literalTerminals {
		for _, spelling := range terminal.forms {
			tokens, err := lex("keyword", []byte(spelling), g)
			if err != nil || len(tokens) != 2 || tokens[0].kind == tokenID || !IsKeyword(spelling) {
				t.Errorf("reserved spelling %q disagrees with lexer: %v, %v", spelling, tokens, err)
			}
		}
	}
}
