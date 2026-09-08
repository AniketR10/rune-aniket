; Function calls

(call_expression
  function: (identifier) @function)

(call_expression
  function: (field_expression
    field: (field_identifier) @function.method))

(call_expression
  function: (scoped_identifier
    name: (identifier) @function))

(macro_invocation
  macro: (identifier) @function.builtin)

; Function definitions

(function_item
  name: (identifier) @function)

(function_signature_item
  name: (identifier) @function)

; Identifiers

(type_identifier) @type
(primitive_type) @type
(field_identifier) @property

(shorthand_field_initializer
  (identifier) @property)

; Constants

((identifier) @constant
  (#match? @constant "^[A-Z][A-Z0-9_]*$"))

(self) @variable

; Literals

(string_literal) @string
(raw_string_literal) @string
(char_literal) @string
(escape_sequence) @escape

(integer_literal) @number
(float_literal) @number

(boolean_literal) @constant.builtin

; Comments

(line_comment) @comment
(block_comment) @comment

; Keywords

[
  "as"
  "async"
  "await"
  "break"
  "const"
  "continue"
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
  "pub"
  "ref"
  "return"
  "static"
  "struct"
  "trait"
  "type"
  "union"
  "unsafe"
  "use"
  "where"
  "while"
  (crate)
  (mutable_specifier)
] @keyword

; Operators

[
  "+"
  "-"
  "*"
  "/"
  "%"
  "="
  "=="
  "!="
  "<"
  ">"
  "<="
  ">="
  "&&"
  "||"
  "!"
  "&"
  "|"
  "^"
  "->"
  "=>"
] @operator
