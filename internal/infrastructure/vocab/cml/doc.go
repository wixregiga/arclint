// Package cmlvocab maps Context Mapper CML to the project's Ubiquitous Language.
// Native CML declarations carry the concepts that CML models; arclint-owned hint
// entries and comment directives carry aliases, assertions, questions and names
// that have no native representation. Grammar comments can supply fallback
// definitions; ordinary file headers never supply project data. Only arclint
// directives act as explicit comment metadata. The vocabulary constructors own
// domain validation; the adapter reports all projection problems with their
// source positions and never returns a partial language. References written by the adapter link within their context.
//
// Render and Record use syntax.FormatChecked, with the same canonical layout.
// Render creates a complete recording that preserves every constructor-valid
// language, ignoring source lines and the recording path. Repository names use
// native declarations only when their spelling and model-wide uniqueness satisfy
// Context Mapper's validators; otherwise the root's hint carries the exact name.
//
// Fresh BoundedContext bodies place services before aggregates, then the context's
// Module and question directives. Sync moves a direct services block containing
// a documented service before every direct Module and Aggregate: Xtext consumes
// a doc string as continuation of their selected repeated member.
// Context Mapper may warn about Modules, identity attributes naming value objects,
// duplicate domain object names across contexts, and lowercase object/service names.
//
// Callers edit vocabulary values and pass them to Repository.Record. It edits the
// existing syntax AST in place, retaining authored CML constructs and ordinary
// comments outside the projection, verifies the requested language by reading it
// back, and replaces the file atomically. Ordered vocabulary sections fill the
// existing modeled declaration slots and carry their comments with them. A
// declaration the file does not hold yet is written beside the one that precedes
// it in the requested order, and an invariant or assertion beside the owned
// operation that precedes it, so an insertion moves no other declaration; hint
// entries the adapter adds follow the order a fresh recording writes them. A
// Module named after its context goes with its last member when the edit empties
// it and it carries nothing else, and a body whose last assigned field an edit
// clears loses its braces. Surviving
// declarations use their fresh-write home when an enclosing aggregate disappears.
// Optional references to removed declarations are cleared; required references
// refuse the edit with the referencing construct's source line. A native Repository
// retains its authored body on rename, and refuses migration to a hint when that
// body cannot be represented by CML under the requested name. Authored optional
// tokens and keyword alternatives survive edits; layout becomes canonical once.
// Recording an unchanged language leaves
// the original bytes untouched. Parse offers the same validated projection for
// callers that already hold the recording's bytes.
package cmlvocab
