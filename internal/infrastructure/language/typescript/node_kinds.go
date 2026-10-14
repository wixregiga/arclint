package typescript

// Node kinds from the pinned TypeScript and TSX grammars.
const (
	tsIdentifier                   = "identifier"
	tsArrowFunction                = "arrow_function"
	tsFunction                     = "function"
	tsFunctionExpression           = "function_expression"
	tsRequiredParameter            = "required_parameter"
	tsOptionalParameter            = "optional_parameter"
	tsAbstractClassDeclaration     = "abstract_class_declaration"
	tsClassDeclaration             = "class_declaration"
	tsClassExpression              = "class"
	tsInterfaceDeclaration         = "interface_declaration"
	tsMethodSignature              = "method_signature"
	tsPropertySignature            = "property_signature"
	tsTypeAliasDeclaration         = "type_alias_declaration"
	tsFunctionDeclaration          = "function_declaration"
	tsGeneratorFunctionDeclaration = "generator_function_declaration"
	tsMethodDefinition             = "method_definition"
	tsVariableDeclarator           = "variable_declarator"
	tsModule                       = "module"
	tsMemberExpression             = "member_expression"
	tsPublicFieldDefinition        = "public_field_definition"
	tsFieldDefinition              = "field_definition"
	tsStatementBlock               = "statement_block"
	tsNewExpression                = "new_expression"
	tsTypeIdentifier               = "type_identifier"
	tsGenericType                  = "generic_type"
	tsFunctionType                 = "function_type"
	tsConditionalType              = "conditional_type"
	tsLookupType                   = "lookup_type"
	tsIndexedAccessType            = "indexed_access_type"
	tsObjectType                   = "object_type"
	tsMappedTypeClause             = "mapped_type_clause"
	tsParenthesizedType            = "parenthesized_type"
	tsReadonlyType                 = "readonly_type"
	tsNamespaceImport              = "namespace_import"
	tsConstructor                  = "constructor"
	tsCallExpression               = "call_expression"
	tsProgram                      = "program"
	tsInternalModule               = "internal_module"
	tsLexicalDeclaration           = "lexical_declaration"
	tsVariableDeclaration          = "variable_declaration"
	tsImportStatement              = "import_statement"
	tsExportStatement              = "export_statement"
	tsTypeParameter                = "type_parameter"
)

// Spelled builtin identities shared by syntax and resolution.
const (
	tsNumber = "number"
)

const (
	tsBuiltinArray     = "Array"
	tsBuiltinObject    = "Object"
	tsBuiltinFunction  = "Function"
	tsBodyField        = "body"
	tsBooleanType      = "boolean"
	tsDefaultKeyword   = "default"
	tsFieldDeclaration = "field"
	tsNameField        = "name"
	tsObjectKeyword    = "object"
	tsParametersField  = "parameters"
	tsPatternField     = "pattern"
	tsPropertyField    = "property"
	tsStringType       = "string"
	tsThis             = "this"
	tsTypeField        = "type"
	tsValueField       = "value"
	tsVarKeyword       = "var"
)

const tsGlobalNamespace = "global"

const (
	tsNullType             = "null"
	tsTupleType            = "tuple_type"
	tsClassBody            = "class_body"
	tsComputedName         = "computed_property_name"
	tsTypeAnnotation       = "type_annotation"
	tsConstructorType      = "constructor_type"
	tsNestedTypeIdentifier = "nested_type_identifier"
	tsBigintType           = "bigint"
	tsVoidType             = "void"
)

const (
	tsReadonlyArray = "ReadonlyArray"
	tsUndefinedType = "undefined"
)
