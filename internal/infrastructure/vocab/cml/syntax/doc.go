// Package syntax parses and formats the complete Context Mapper CML grammar
// pinned under grammar using a Go standard-library recursive descent interpreter.
// Parse reads a UTF-8 model; ParseFile adds the filename to source diagnostics.
// File aliases ContextMappingModel. Grammar features are exported fields, repeated
// features are slices, unions are sealed interfaces, and cross-references are Ref
// values. This adapter parses syntax without linking references or judging domain
// semantics. Repeated unordered-group members stay contiguous and their loop
// continuation follows Xtext/ANTLR FIRST-token decisions, including shared doc
// strings. IsKeyword exposes the lexer tables for safe identifier spelling.
//
// Callers edit AST fields and slices directly, then use FormatChecked to check
// grammar structure and reparse the result. Validate performs that same check;
// Format is the byte-only convenience API. New nodes can leave positions zero.
// Parsed declaration positions retain source order across feature slices; new
// declarations use canonical composition order (Domain, ContextMap, contexts;
// services, aggregates, Modules inside a context), otherwise grammar order. Strings hold unescaped values. Node.Spellings
// keeps escaped datatype spellings separately: Type "String" with spelling
// "^String" names a datatype rather than the builtin. Update or delete that
// spelling when changing its meaning. Node.Present records explicitly assigned
// zero values, using grammar feature names; Present["doc"] emits an empty doc.
// Node.ExplicitBody retains braces the author wrote around nothing, or around
// optional syntax that no assigned field represents, such as a keyword kept at
// its default. Braces around assigned fields alone print while one of them has
// a value and go with the last; clear the flag to omit any other emptied body.
// New empty optional bodies have no braces.
// Nodes also retain each occurrence's optional punctuation, keyword alternatives,
// repeated clause grouping, primitive clause order and scalar token spellings.
// Editing assigned fields updates their values; unchanged token choices survive.
// New nodes choose explicit property equals signs, void results and operation
// parentheses. No parse derivation is retained for those choices.
//
// Node.LeadingComments owns comments directly before a declaration; a block owns
// comments before its closing brace in Node.TrailingComments. File also owns its
// leading and trailing runs. The first comment run is the file comment; a blank
// line separates it from the first declaration's comments. Comments on primitive
// clauses attach to the following node or to the block's trailing comments.
// Comment and TopComment expose the corresponding text. Callers edit comment
// slices to change runs. Source positions describe the original input and do not
// change during editing or formatting. Parse trees are discarded after attachment.
//
// Formatting is deterministic, uses two-space indentation, and separates
// declarations in composition blocks while keeping domain object members together.
// Doc strings sit above their declarations, contains lists use one line per
// authored clause, and bracketed relationship arrows and roles are compact.
// Strings are escaped and identifiers are keyword-safe. Relationships retain
// Notation in grammar-alternative order, including participant order, arrow
// direction and bracket placement. New relationships can select Partnership 4,
// SharedKernel 4, or directional 2 or 3 for keyword notation. HTTPMethod follows
// Go's initialism convention for the grammar's HttpMethod.
package syntax
