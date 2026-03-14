package highlight

// Highlight queries for each language.
// These use tree-sitter's query syntax to capture node types as token names.

var goHighlightQuery = `
(comment) @comment
(interpreted_string_literal) @string
(raw_string_literal) @string
(rune_literal) @string
(int_literal) @number
(float_literal) @number
(imaginary_literal) @number
(true) @number
(false) @number
(nil) @number

[
  "break"
  "case"
  "chan"
  "const"
  "continue"
  "default"
  "defer"
  "else"
  "fallthrough"
  "for"
  "func"
  "go"
  "goto"
  "if"
  "import"
  "interface"
  "map"
  "package"
  "range"
  "return"
  "select"
  "struct"
  "switch"
  "type"
  "var"
] @keyword

(function_declaration name: (identifier) @function)
(method_declaration name: (field_identifier) @function)
(call_expression function: (identifier) @function)
(call_expression function: (selector_expression field: (field_identifier) @function))

(type_identifier) @type
(type_spec name: (type_identifier) @type)

(parameter_declaration name: (identifier) @variable)
(short_var_declaration left: (expression_list (identifier) @variable))
`

var pythonHighlightQuery = `
(comment) @comment
(string) @string

[
  "and"
  "as"
  "assert"
  "async"
  "await"
  "break"
  "class"
  "continue"
  "def"
  "del"
  "elif"
  "else"
  "except"
  "finally"
  "for"
  "from"
  "global"
  "if"
  "import"
  "in"
  "is"
  "lambda"
  "nonlocal"
  "not"
  "or"
  "pass"
  "raise"
  "return"
  "try"
  "while"
  "with"
  "yield"
] @keyword

(function_definition name: (identifier) @function)
(call function: (identifier) @function)
(call function: (attribute attribute: (identifier) @function))

(class_definition name: (identifier) @type)

(integer) @number
(float) @number

(true) @number
(false) @number
(none) @number

(identifier) @variable
`

var jsHighlightQuery = `
(comment) @comment
(string) @string
(template_string) @string

[
  "async"
  "await"
  "break"
  "case"
  "catch"
  "class"
  "const"
  "continue"
  "debugger"
  "default"
  "delete"
  "do"
  "else"
  "export"
  "extends"
  "finally"
  "for"
  "function"
  "if"
  "import"
  "in"
  "instanceof"
  "let"
  "new"
  "of"
  "return"
  "static"
  "switch"
  "throw"
  "try"
  "typeof"
  "var"
  "void"
  "while"
  "with"
  "yield"
] @keyword

(function_declaration name: (identifier) @function)
(call_expression function: (identifier) @function)
(call_expression function: (member_expression property: (property_identifier) @function))
(method_definition name: (property_identifier) @function)
(arrow_function)

(number) @number
(true) @number
(false) @number
(null) @number
(undefined) @number
`

var rustHighlightQuery = `
(line_comment) @comment
(block_comment) @comment
(string_literal) @string
(char_literal) @string
(raw_string_literal) @string

[
  "as"
  "async"
  "await"
  "break"
  "const"
  "continue"
  "crate"
  "dyn"
  "else"
  "enum"
  "extern"
  "fn"
  "for"
  "if"
  "impl"
  "in"
  "let"
  "loop"
  "match"
  "mod"
  "move"
  "mut"
  "pub"
  "ref"
  "return"
  "self"
  "static"
  "struct"
  "super"
  "trait"
  "type"
  "unsafe"
  "use"
  "where"
  "while"
] @keyword

(function_item name: (identifier) @function)
(call_expression function: (identifier) @function)
(call_expression function: (field_expression field: (field_identifier) @function))

(type_identifier) @type
(primitive_type) @type

(integer_literal) @number
(float_literal) @number
(boolean_literal) @number

(identifier) @variable
`
