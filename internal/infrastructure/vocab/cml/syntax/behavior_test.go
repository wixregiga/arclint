package syntax

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestRelationshipsEveryNotation(t *testing.T) {
	cases := []struct {
		source      string
		kind        string
		notation    Notation
		left, right string
	}{
		{`Alpha [P] <-> [P] Beta`, "Partnership", 0, "Alpha", "Beta"},
		{`[P] Alpha <-> [P] Beta`, "Partnership", 1, "Alpha", "Beta"},
		{`Alpha [P] <-> Beta [P]`, "Partnership", 2, "Alpha", "Beta"},
		{`[P] Alpha <-> Beta [P]`, "Partnership", 3, "Alpha", "Beta"},
		{`Alpha Partnership Beta`, "Partnership", 4, "Alpha", "Beta"},
		{`Alpha [SK] <-> [SK] Beta`, "SharedKernel", 0, "Alpha", "Beta"},
		{`[SK] Alpha <-> [SK] Beta`, "SharedKernel", 1, "Alpha", "Beta"},
		{`Alpha [SK] <-> Beta [SK]`, "SharedKernel", 2, "Alpha", "Beta"},
		{`[SK] Alpha <-> Beta [SK]`, "SharedKernel", 3, "Alpha", "Beta"},
		{`Alpha Shared-Kernel Beta`, "SharedKernel", 4, "Alpha", "Beta"},
		{`Alpha <-> Beta`, "SharedKernel", 5, "Alpha", "Beta"},
		{`Alpha [U,PL,OHS] -> [D,ACL,CF] Beta`, "UpstreamDownstreamRelationship", 0, "Alpha", "Beta"},
		{`Beta [D,ACL,CF] <- [U,PL,OHS] Alpha`, "UpstreamDownstreamRelationship", 1, "Alpha", "Beta"},
		{`Alpha [PL,OHS] Upstream-Downstream [ACL,CF] Beta`, "UpstreamDownstreamRelationship", 2, "Alpha", "Beta"},
		{`Beta [ACL,CF] Downstream-Upstream [PL,OHS] Alpha`, "UpstreamDownstreamRelationship", 3, "Alpha", "Beta"},
		{`Alpha [U,S,PL,OHS] -> [D,C,ACL,CF] Beta`, "CustomerSupplierRelationship", 0, "Alpha", "Beta"},
		{`Beta [D,C,ACL,CF] <- [U,S,PL,OHS] Alpha`, "CustomerSupplierRelationship", 1, "Alpha", "Beta"},
		{`Beta [ACL,CF] Customer-Supplier [PL,OHS] Alpha`, "CustomerSupplierRelationship", 2, "Alpha", "Beta"},
		{`Alpha [PL,OHS] Supplier-Customer [ACL,CF] Beta`, "CustomerSupplierRelationship", 3, "Alpha", "Beta"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			file, err := Parse([]byte("ContextMap {" + tc.source + " : Named {implementationTechnology \"Go\"}}"))
			if err != nil {
				t.Fatal(err)
			}
			r := reflect.ValueOf(file.Map.Relationships[0]).Elem()
			if r.Type().Name() != tc.kind || Notation(r.FieldByName("Notation").Int()) != tc.notation {
				t.Fatalf("wrong relationship: %+v", file.Map.Relationships[0])
			}
			leftKey, rightKey := "Participant1", "Participant2"
			if strings.Contains(tc.kind, "Relationship") {
				leftKey, rightKey = "Upstream", "Downstream"
			}
			if r.FieldByName(leftKey).Interface().(Ref).Name != tc.left || r.FieldByName(rightKey).Interface().(Ref).Name != tc.right {
				t.Fatal("participant order changed")
			}
			assertRoundTrip(t, file)
		})
	}
}

func TestUnorderedGroupsDoNotResume(t *testing.T) {
	// Xtext 2.32 UnorderedGroupHelper.State.select marks predicate[index] true;
	// returnFromSelection clears selected, and canSelect then rejects that member.
	// AbstractAntlrGrammarWithActionsGenerator._ebnf2(UnorderedGroup) places each
	// member's complete ebnf (including * or +) between select and returnFromSelection.
	// Thus repetitions are contiguous and cannot resume after another member.
	cases := []struct {
		source string
		valid  bool
	}{
		{`Domain Dom BoundedContext Alpha BoundedContext Beta`, true},
		{`BoundedContext Alpha Domain Dom BoundedContext Beta`, false},
		{`ValueRegister Reg {Value First Value Second ValueCluster Cluster {core PRIVACY}}`, true},
		{`ValueRegister Reg {Value First ValueCluster Cluster {core PRIVACY} Value Second}`, false},
		{`ValueRegister Reg {ValueEpic Epic {As a Stake I value "v" as demonstrated in realization of "one" realization of "two" reduction of "harm"}}`, true},
		{`ValueRegister Reg {ValueEpic Epic {As a Stake I value "v" as demonstrated in realization of "one" reduction of "harm" realization of "two"}}`, false},
		{`ValueRegister Reg {ValueCluster Cluster {demonstrator "one" core PRIVACY demonstrator "two"}}`, false},
		{`BoundedContext Alpha {type TEAM responsibilities "one" domainVisionStatement "v"}`, true},
		{`BoundedContext Alpha {type TEAM type SYSTEM}`, false},
	}
	for _, tc := range cases {
		_, err := Parse([]byte(tc.source))
		if (err == nil) != tc.valid {
			t.Errorf("valid=%v for %s: %s", tc.valid, tc.source, err)
		}
	}
}

func TestTerminalPrecedence(t *testing.T) {
	// AbstractAntlrGrammarGenerator.compileLexer emits keyword rules before
	// GrammarUtil.allTerminalRules, which preserves declaration and inheritance order.
	// ANTLR 3 AssignTokenTypesBehavior.trackTokenRule (lines 137-153) aliases
	// a singleton literal terminal in a combined grammar: OPEN/CLOSE share braces.
	// Multi-alternative OPPOSITE and DELEGATE have no such alias, so the two
	// inherited spellings shadowed by mapping keywords are unreachable.
	valid := []string{
		`BoundedContext Alpha {Aggregate Group {Entity Obj {- Other ref opposite back}}}`,
		`BoundedContext Alpha {Aggregate Group {Service Svc {op() => Repo.save;}}}`,
		`BoundedContext ^Map {Aggregate ^not {Entity ^String {String ^description}}}`,
		`ContextMap {Alpha <-> Beta}`,
		`BoundedContext Alpha {Application {Flow Work {command Cmd delegates to Group emits event Ev}}}`,
	}
	invalid := []string{
		`BoundedContext Alpha {Aggregate Group {Entity Obj {- Other ref <-> back}}}`,
		`BoundedContext Alpha {Aggregate Group {Service Svc {op() delegates to Repo.save;}}}`,
		`BoundedContext Map`,
		`BoundedContext Alpha {Aggregate Group {Entity Obj {String description}}}`,
	}
	for _, source := range valid {
		file, err := Parse([]byte(source))
		if err != nil {
			t.Errorf("valid input: %s", err)
		} else {
			assertRoundTrip(t, file)
		}
	}
	for _, source := range invalid {
		if _, err := Parse([]byte(source)); err == nil {
			t.Errorf("shadowed terminal accepted: %s", source)
		}
	}
	g, err := cmlGrammar()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := lex("precedence", []byte(`-- - not notX Map MapX ^Map <-> delegates to =>`), g)
	if err != nil {
		t.Fatal(err)
	}
	kinds := []string{"ASC", "REF", "NOT", "ID", "MAP_COLLECTION_TYPE", "ID", "ID", "<->", "delegates to", "DELEGATE", "EOF"}
	for i, want := range kinds {
		if tokens[i].kind != want {
			t.Errorf("token %d: got %s, want %s", i, tokens[i].kind, want)
		}
	}
}

func TestStrings(t *testing.T) {
	cases := []struct{ literal, value string }{
		{`"plain"`, "plain"},
		{`'single'`, "single"},
		{`"\b\t\n\f\r\"\'\\"`, "\b\t\n\f\r\"'\\"},
		{`'\"\'\\'`, "\"'\\"},
		{`"\u0041\u0000\u20ac\ud83d\ude00"`, "A\x00€😀"},
		{"\"line1\nline2\"", "line1\nline2"},
		{`"\ud800"`, string([]byte{0xed, 0xa0, 0x80})},
		{`"\udc00"`, string([]byte{0xed, 0xb0, 0x80})},
	}
	for _, tc := range cases {
		file, err := Parse([]byte("import " + tc.literal))
		if err != nil {
			t.Fatal(err)
		}
		if file.Imports[0].ImportURI != tc.value {
			t.Errorf("%s: got %q want %q", tc.literal, file.Imports[0].ImportURI, tc.value)
		}
		assertRoundTrip(t, file)
	}
}

func TestExactErrors(t *testing.T) {
	cases := []struct {
		source               string
		offset, line, column int
		message              string
	}{
		{`$`, 0, 1, 1, `expected CML token; found "$" (ANY_OTHER)`},
		{"import \"\xed\"", 8, 1, 9, "expected UTF-8 source; found invalid byte 0xed"},
		{"\n/* unclosed", 1, 2, 1, "expected */ to close block comment; found end of file"},
		{`import "open`, 7, 1, 8, "expected closing quote for string; found end of file"},
		{`import "a\q"`, 9, 1, 10, `expected valid STRING escape; found \q`},
		{`import "\u12x4"`, 8, 1, 9, `expected four hexadecimal digits after \u`},
		{`import "\u12"`, 8, 1, 9, `expected four hexadecimal digits after \u`},
		{`import "\123"`, 8, 1, 9, `expected valid STRING escape; found \1`},
		{`2147483648`, 0, 1, 1, "expected 32-bit INT; found 2147483648"},
		{`BoundedContext TEAM`, 15, 1, 16, `expected ID; found "TEAM" (TEAM)`},
		{"\r\nBoundedContext TEAM", 17, 2, 16, `expected ID; found "TEAM" (TEAM)`},
		{"\rBoundedContext TEAM", 16, 2, 16, `expected ID; found "TEAM" (TEAM)`},
		{"import 'a'\nBoundedContext", 25, 2, 15, `expected ID; found end of file`},
	}
	for _, tc := range cases {
		_, err := ParseFile("bad.cml", []byte(tc.source))
		var parseErr *Error
		if !errors.As(err, &parseErr) {
			t.Errorf("%q: no precise Error: %v", tc.source, err)
			continue
		}
		want := Pos{tc.offset, tc.line, tc.column}
		if parseErr.Pos != want || parseErr.Message != tc.message || parseErr.File != "bad.cml" {
			t.Errorf("%q: got %+v (%s), want %+v: %s", tc.source, parseErr, err, want, tc.message)
		}
	}
}

func TestCommentsAndPositions(t *testing.T) {
	source := []byte("// file\n\n// context\nBoundedContext Alpha {\n // group\n \"doc\" Aggregate Group {\n  // object\n  Entity Obj {\n   // trailing object\n  }\n  // trailing group\n }\n // trailing context\n}\n// trailing file")
	file, err := ParseFile("comments.cml", source)
	if err != nil {
		t.Fatal(err)
	}
	bc := file.BoundedContexts[0]
	ag := bc.Aggregates[0]
	obj := ag.DomainObjects[0].(*Entity)
	if file.TopComment != "// file" || bc.Comment != "// context" || ag.Comment != "// group" || obj.Comment != "// object" {
		t.Fatalf("named comment features: %+v %+v %+v", bc, ag, obj)
	}
	if len(file.LeadingComments) != 1 || len(bc.LeadingComments) != 1 || len(ag.LeadingComments) != 1 || len(obj.LeadingComments) != 1 || len(obj.TrailingComments) != 1 || len(ag.TrailingComments) != 1 || len(bc.TrailingComments) != 1 || len(file.TrailingComments) != 1 {
		t.Fatal("comment ownership changed")
	}
	if bc.Pos.Line != 4 || bc.NamePos.Line != 4 || bc.NamePos.Column != 16 || ag.Pos.Line != 6 || ag.NamePos.Line != 6 {
		t.Fatalf("node provenance: BC=%+v name=%+v aggregate=%+v", bc.Pos, bc.NamePos, ag.Pos)
	}
	if ag.Pos.Offset != bytes.Index(source, []byte(`"doc"`)) {
		t.Fatal("doc token did not anchor Aggregate byte offset")
	}
	assertRoundTrip(t, file)
	commentsOnly, err := Parse([]byte("// only\n/* final */"))
	if err != nil {
		t.Fatal(err)
	}
	if len(commentsOnly.TrailingComments) != 2 {
		t.Fatal("comment-only file has no following node")
	}
	assertRoundTrip(t, commentsOnly)
}

func TestEditAndBuildThroughAPI(t *testing.T) {
	file, err := Parse([]byte(`BoundedContext Old {Aggregate Gone {Entity Remove} Aggregate Keep {Entity Obj {String old def oldOp(); Repository OldRepo} ValueObject OldValue DomainEvent OldEvent Service OldService {old();} ModuleBad attr}}`))
	// ModuleBad attr is an invalid aggregate member; do not allow a malformed model
	// to become a partially usable tree.
	if err == nil || file != nil {
		t.Fatal("invalid input became a model")
	}
	file, err = Parse([]byte(`BoundedContext Old {Module GoneModule Aggregate Gone {Entity Remove} Aggregate Keep {Entity Obj {String old def oldOp(); Repository OldRepo} ValueObject OldValue DomainEvent OldEvent Service OldService {old();}}}`))
	if err != nil {
		t.Fatal(err)
	}
	bc := file.BoundedContexts[0]
	bc.Name = "TEAM"
	bc.Modules = nil
	bc.Aggregates = bc.Aggregates[1:]
	ag := bc.Aggregates[0]
	obj := ag.DomainObjects[0].(*Entity)
	obj.Attributes[0].Name = "description"
	obj.Attributes[0].Type = "int"
	obj.Operations = nil
	obj.Repository = &Repository{Name: "FreshRepo", Operations: []*RepositoryOperation{{Name: "save"}}}
	ag.DomainObjects = append(ag.DomainObjects[:1], &ValueObject{Name: "FreshValue", Attributes: []*Attribute{{Type: "String", Name: "code"}}}, &DomainEvent{Name: "FreshEvent"})
	ag.Services = []*Service{{Name: "FreshService", Operations: []*ServiceOperation{{Name: "run", ReturnType: &ComplexType{DomainObjectType: Ref{Name: "FreshValue"}}}}}}
	bc.Modules = []*SculptorModule{{Name: "FreshModule", Aggregates: []*Aggregate{{Name: "Nested"}}}}
	file.BoundedContexts = append(file.BoundedContexts, &BoundedContext{Name: "Beta"})
	file.Map = &ContextMap{BoundedContexts: []Ref{{Name: "TEAM"}, {Name: "Beta"}}, Relationships: []Relationship{&CustomerSupplierRelationship{Upstream: Ref{Name: "TEAM"}, Downstream: Ref{Name: "Beta"}, Notation: 3, UpstreamRoles: []UpstreamRole{"PL"}, DownstreamRoles: []DownstreamRole{"ACL"}}}}
	output, err := FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte("BoundedContext ^TEAM")) || !bytes.Contains(output, []byte("int ^description")) || bytes.Contains(output, []byte("oldOp")) || bytes.Contains(output, []byte("OldService")) {
		t.Fatalf("edits not printed:\n%s", output)
	}
	again, err := Parse(output)
	if err != nil {
		t.Fatal(err)
	}
	assertRoundTrip(t, again)
	emptyStrings := &File{ValueRegisters: []*ValueRegister{{Name: "Reg", ValueNarratives: []*ValueNarrative{{Name: "Narrative"}}}}}
	if _, err := FormatChecked(emptyStrings); err != nil {
		t.Fatalf("mandatory STRING fields may be empty: %s", err)
	}
}

func assertRoundTrip(t *testing.T, file *File) {
	t.Helper()
	output, err := FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(output)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(semantic(reflect.ValueOf(file)), semantic(reflect.ValueOf(again))) {
		t.Fatalf("semantic round trip changed for:\n%s", output)
	}
	if !bytes.Equal(output, formatted(t, again)) {
		t.Fatalf("formatter is not a fixed point:\n%s", output)
	}
}

// formatted is FormatChecked for a tree a test expects to be valid.
func formatted(t *testing.T, file *File) []byte {
	t.Helper()
	output, err := FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func TestLargeModel(t *testing.T) {
	t.Parallel()
	// Large sibling lists must not count as grammar nesting or take quadratic
	// temporary storage. This also checks that speculative return-type lookahead
	// does not corrupt earlier siblings when its backing slices are reused.
	fields := 4000
	if !exhaustive() {
		fields = 1000
	}
	var source strings.Builder
	source.WriteString("BoundedContext Large {Aggregate Group {Entity Obj {")
	for i := 0; i < fields; i++ {
		source.WriteString("String field")
		source.WriteString(strconv.Itoa(i))
		source.WriteByte('\n')
	}
	source.WriteString("}}}")
	file, err := Parse([]byte(source.String()))
	if err != nil {
		t.Fatal(err)
	}
	attrs := file.BoundedContexts[0].Aggregates[0].DomainObjects[0].(*Entity).Attributes
	last := "field" + strconv.Itoa(fields-1)
	if len(attrs) != fields || attrs[0].Name != "field0" || attrs[fields-1].Name != last {
		t.Fatal("large sibling list was corrupted")
	}
	assertRoundTrip(t, file)
}

func TestExposedAggregatesComment(t *testing.T) {
	source := []byte("ContextMap {Alpha -> Beta {exposedAggregates Group,Other // exposed\n implementationTechnology \"Go\"}}")
	file, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	r := file.Map.Relationships[0].(*UpstreamDownstreamRelationship)
	if r.ExposedAggregatesComment != "// exposed" {
		t.Fatalf("named exposedAggregatesComment missing: %q", r.ExposedAggregatesComment)
	}
	assertRoundTrip(t, file)
}

func TestOptionalRelationshipBracketsAndMarkers(t *testing.T) {
	headers := []string{
		`Alpha -> Beta`, `Alpha [U] -> [D] Beta`, `Beta [D] <- [U] Alpha`,
		`Alpha [PL] -> [ACL] Beta`, `Alpha [U,PL] -> [ACL] Beta`, `Beta [D,ACL] <- [PL] Alpha`,
		`Alpha [] Upstream-Downstream [] Beta`, `Beta [] Downstream-Upstream [] Alpha`,
		`Alpha [S] -> [C] Beta`, `Alpha [U,S] -> [D,C] Beta`, `Beta [C] <- [S] Alpha`,
		`Beta [] Customer-Supplier [] Alpha`, `Alpha [] Supplier-Customer [] Beta`,
	}
	for _, header := range headers {
		t.Run(header, func(t *testing.T) {
			file, err := Parse([]byte("ContextMap {" + header + "}"))
			if err != nil {
				t.Fatal(err)
			}
			output, err := FormatChecked(file)
			if err != nil {
				t.Fatal(err)
			}
			compact := func(s string) string {
				return strings.Map(func(r rune) rune {
					if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
						return -1
					}
					return r
				}, s)
			}
			if !strings.Contains(compact(string(output)), compact(header)) {
				t.Fatalf("author's header changed: %s -> %s", header, output)
			}
			assertRoundTrip(t, file)
		})
	}
}

func TestEscapedDatatypeAndDeclarationOrder(t *testing.T) {
	source := []byte(`BoundedContext Alpha {
 Aggregate Group {
  Entity Obj { ^String id key; }
  DomainEvent FirstEvent
  Service FirstService
  DomainEvent MiddleEvent
 }
 Module Mod {
  DomainEvent LastEvent
 }
}`)
	file, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	attribute := file.BoundedContexts[0].Aggregates[0].DomainObjects[0].(*Entity).Attributes[0]
	if attribute.Type != "String" || attribute.Spellings["type"] != "^String" {
		t.Fatalf("escaped type became builtin: %q", attribute.Type)
	}
	output, err := FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte("^String id key")) {
		t.Fatalf("escaped datatype was lost: %s", output)
	}
	before := -1
	for _, name := range []string{"FirstEvent", "FirstService", "MiddleEvent", "LastEvent"} {
		at := bytes.Index(output, []byte(name))
		if at <= before {
			t.Fatalf("declarations were reordered: %s", output)
		}
		before = at
	}
	assertRoundTrip(t, file)
}

func TestKeywordSTRINGAndStringTerminalAreDistinct(t *testing.T) {
	// DiscriminatorType has a keyword named STRING. Its keyword token is not the
	// inherited STRING terminal, even though their readable names are identical.
	for _, source := range []string{`import STRING`, `UseCase Req {actor STRING}`, `BoundedContext Alpha {Aggregate Group {Entity Obj {hint = STRING}}}`} {
		if _, err := Parse([]byte(source)); err == nil {
			t.Errorf("keyword STRING became a STRING terminal: %s", source)
		}
	}
	file, err := Parse([]byte(`import "STRING" BoundedContext Alpha {Aggregate Group {Entity Obj {discriminatorType = STRING}}}`))
	if err != nil {
		t.Fatal(err)
	}
	assertRoundTrip(t, file)
}

func TestSyntacticErrorClassesHaveExactPositions(t *testing.T) {
	cases := []struct {
		source         string
		offset, column int
		expected       string
	}{
		{`}`, 0, 1, "expected"},
		{`BoundedContext Alpha {`, 22, 23, "CLOSE"},
		{`BoundedContext Alpha {type TEAM type SYSTEM}`, 32, 33, "CLOSE"},
		{`BoundedContext Alpha {type UNKNOWN}`, 27, 28, `"FEATURE"`},
		{`ContextMap {contains Alpha,}`, 27, 28, "ID for BoundedContext"},
		{`BoundedContext Alpha {Aggregate Group {Entity Obj {- Other;}}}`, 58, 59, "ID"},
		{`BoundedContext Alpha {Aggregate Group {Service Svc {op()}}}`, 56, 57, `";"`},
		{`ContextMap {Entity Foo}`, 12, 13, "CLOSE"},
		{`BoundedContext Alpha {Aggregate Group {Entity Obj {List<String field}}}`, 63, 64, `">"`},
	}
	for _, tc := range cases {
		_, err := ParseFile("syntax.cml", []byte(tc.source))
		var failure *Error
		if !errors.As(err, &failure) {
			t.Errorf("missing syntax error for %s: %s", tc.source, err)
			continue
		}
		if failure.Pos != (Pos{tc.offset, 1, tc.column}) || !strings.Contains(failure.Message, tc.expected) {
			t.Errorf("%s: got %+v, expected offset=%d col=%d and %s", tc.source, failure, tc.offset, tc.column, tc.expected)
		}
	}
}

func TestInvalidConstructedValuesAreRejected(t *testing.T) {
	files := []*File{
		{BoundedContexts: []*BoundedContext{{Node: Node{LeadingComments: []Comment{{Text: "/* comment */ BoundedContext Injected"}}}, Name: "Alpha"}}},
		{BoundedContexts: []*BoundedContext{{Name: "Alpha } BoundedContext Injected {"}}},
		{Map: &ContextMap{BoundedContexts: []Ref{{Name: "Alpha,Beta"}}}},
		{Imports: []*Import{{ImportURI: string([]byte{0xff})}}},
	}
	for _, file := range files {
		if _, err := FormatChecked(file); err == nil {
			t.Fatal("invalid constructed value produced CML")
		}
	}
}

func TestConstructedCommentFeatures(t *testing.T) {
	file := &File{TopComment: "// file", BoundedContexts: []*BoundedContext{{Name: "Alpha", Comment: "/* context */"}}}
	output, err := FormatChecked(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte("// file")) || !bytes.Contains(output, []byte("/* context */")) {
		t.Fatalf("constructed comment features were lost: %s", output)
	}
	parsed, err := Parse(output)
	if err != nil {
		t.Fatal(err)
	}
	assertRoundTrip(t, parsed)
}
