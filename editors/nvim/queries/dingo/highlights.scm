; Keywords
[
  "package"
  "import"
  "func"
  "return"
  "type"
  "struct"
  "interface"
  "map"
  "chan"
  "for"
  "range"
  "switch"
  "case"
  "default"
  "select"
  "break"
  "continue"
  "goto"
  "fallthrough"
  "defer"
  "go"
  "if"
  "else"
  "var"
  "const"
] @keyword

; Dingo-specific keywords
[
  "enum"
  "match"
  "let"
  "guard"
  "where"
] @keyword

; Operators
[
  "="
  ":="
  "+"
  "-"
  "*"
  "/"
  "%"
  "&"
  "|"
  "^"
  "!"
  "<"
  ">"
  "=="
  "!="
  "<="
  ">="
  "&&"
  "||"
  "<<"
  ">>"
  "<-"
] @operator

; Dingo special operators
"=>" @operator
"?." @operator
"??" @operator
(error_propagation "?" @operator)

; Lambda pipes (special highlighting)
(rust_style_lambda "|" @punctuation.special)

; Types
(type_spec name: (identifier) @type.definition)
(type_parameter (identifier) @type.parameter)
(generic_type (identifier) @type)

; Dingo enum
(enum_declaration name: (identifier) @type.definition)
(enum_variant name: (identifier) @constructor)

; Match patterns
(match_expression "match" @keyword.control)
(match_arm pattern: (variant_pattern name: (identifier) @constructor))
(wildcard_pattern) @variable.builtin

; Functions
(function_declaration name: (identifier) @function)
(call_expression function: (identifier) @function.call)
(call_expression function: (selector_expression field: (identifier) @function.method.call))

; Parameters
(parameter name: (identifier) @variable.parameter)
(lambda_parameter name: (identifier) @variable.parameter)

; Variables
(let_declaration name: (identifier) @variable)
(var_spec name: (identifier) @variable)
(const_spec name: (identifier) @constant)

; Literals
(int_literal) @number
(float_literal) @number.float
(interpreted_string_literal) @string
(raw_string_literal) @string
(rune_literal) @character
(escape_sequence) @string.escape

; Booleans and nil
(true) @boolean
(false) @boolean
(nil) @constant.builtin

; Comments
(line_comment) @comment
(block_comment) @comment

; Punctuation
[
  "("
  ")"
  "{"
  "}"
  "["
  "]"
] @punctuation.bracket

[
  ","
  ";"
  ":"
  "."
] @punctuation.delimiter

; Import path
(import_spec path: (interpreted_string_literal) @string.special)

; Package name
(package_clause (identifier) @namespace)

; Error propagation expression (special styling)
(error_propagation) @punctuation.special
