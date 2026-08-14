package golang

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/lang"
)

func TestGoFacts(t *testing.T) {
	src := []byte(`package member

import "time"

const MaxLoans = 5

var internalCounter int

type Member struct {
	ID      string
	balance int
	time.Time
}

type Repo interface {
	Find(id string) (Member, error)
	save(m Member) error
}

type MemberID string

func New(id string) *Member { return &Member{ID: id} }

func (m *Member) Blocked() bool {
	return m.balance > 0
}

func (m Member) internalCheck() bool { return false }
`)
	got := Facts("internal/member/member.go", src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	if got.Package != "member" {
		t.Errorf("package = %q", got.Package)
	}
	want := []lang.Decl{
		{Kind: "const", Name: "MaxLoans", Exported: true, StartLine: 5, EndLine: 5},
		{Kind: "var", Name: "internalCounter", StartLine: 7, EndLine: 7},
		{Kind: "struct", Name: "Member", Exported: true, StartLine: 9, EndLine: 13},
		{Kind: "field", Name: "ID", Owner: "Member", Exported: true, StartLine: 10, EndLine: 10},
		{Kind: "field", Name: "balance", Owner: "Member", StartLine: 11, EndLine: 11},
		{Kind: "field", Name: "Time", Owner: "Member", Exported: true, StartLine: 12, EndLine: 12},
		{Kind: "interface", Name: "Repo", Exported: true, StartLine: 15, EndLine: 18},
		{Kind: "method", Name: "Find", Owner: "Repo", Exported: true, StartLine: 16, EndLine: 16,
			Params: []lang.Param{{Name: "id", Type: "string"}}, Results: []string{"Member", "error"}},
		{Kind: "method", Name: "save", Owner: "Repo", StartLine: 17, EndLine: 17,
			Params: []lang.Param{{Name: "m", Type: "Member"}}, Results: []string{"error"}},
		{Kind: "type", Name: "MemberID", Exported: true, StartLine: 20, EndLine: 20},
		{Kind: "func", Name: "New", Exported: true, StartLine: 22, EndLine: 22,
			Params: []lang.Param{{Name: "id", Type: "string"}}, Results: []string{"*Member"}},
		{Kind: "method", Name: "Blocked", Owner: "Member", Exported: true, StartLine: 24, EndLine: 26,
			Results: []string{"bool"}},
		{Kind: "method", Name: "internalCheck", Owner: "Member", StartLine: 28, EndLine: 28,
			Results: []string{"bool"}},
	}
	if !reflect.DeepEqual(got.Decls, want) {
		for i := 0; i < len(got.Decls) || i < len(want); i++ {
			var g, w lang.Decl
			if i < len(got.Decls) {
				g = got.Decls[i]
			}
			if i < len(want) {
				w = want[i]
			}
			marker := "  "
			if !reflect.DeepEqual(g, w) {
				marker = "!!"
			}
			t.Logf("%s got %+v want %+v", marker, g, w)
		}
		t.Fail()
	}
}

// TestGoSignatureFacts pins the M10 signature tier: source-text types,
// multi-name params expanded, variadics flagged, named results dropped
// to their types, unnamed interface-method params kept unnamed.
func TestGoSignatureFacts(t *testing.T) {
	src := []byte(`package sig

import "context"

type Port interface {
	Load(context.Context, string) (map[string]int, error)
}

func Multi(a, b string, opts ...Option) (n int, err error) { return }

func Generic[T any](items []T, fn func(T) bool) *T { return nil }
`)
	got := Facts("sig.go", src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	want := []lang.Decl{
		{Kind: "interface", Name: "Port", Exported: true, StartLine: 5, EndLine: 7},
		{Kind: "method", Name: "Load", Owner: "Port", Exported: true, StartLine: 6, EndLine: 6,
			Params:  []lang.Param{{Type: "context.Context"}, {Type: "string"}},
			Results: []string{"map[string]int", "error"}},
		{Kind: "func", Name: "Multi", Exported: true, StartLine: 9, EndLine: 9,
			Params: []lang.Param{
				{Name: "a", Type: "string"}, {Name: "b", Type: "string"},
				{Name: "opts", Type: "Option", Variadic: true},
			},
			Results: []string{"int", "error"}},
		{Kind: "func", Name: "Generic", Exported: true, StartLine: 11, EndLine: 11,
			Params: []lang.Param{
				{Name: "items", Type: "[]T"}, {Name: "fn", Type: "func(T) bool"},
			},
			Results: []string{"*T"}},
	}
	if !reflect.DeepEqual(got.Decls, want) {
		for i := 0; i < len(got.Decls) || i < len(want); i++ {
			var g, w lang.Decl
			if i < len(got.Decls) {
				g = got.Decls[i]
			}
			if i < len(want) {
				w = want[i]
			}
			marker := "  "
			if !reflect.DeepEqual(g, w) {
				marker = "!!"
			}
			t.Logf("%s got %+v want %+v", marker, g, w)
		}
		t.Fail()
	}
}

func TestGoFactsParseError(t *testing.T) {
	got := Facts("broken.go", []byte("package \x00"))
	if got.ParseError == "" || len(got.Decls) != 0 {
		t.Errorf("broken file: %+v", got)
	}
}
