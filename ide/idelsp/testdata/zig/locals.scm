; Scopes
[
  (source_file)
  (block)
  (function_declaration)
  (struct_declaration)
  (enum_declaration)
  (union_declaration)
  (opaque_declaration)
  (error_set_declaration)
  (test_declaration)
  (comptime_declaration)
  (for_statement)
  (for_expression)
  (while_statement)
  (while_expression)
  (if_statement)
  (if_expression)
  (switch_case)
] @local.scope

; Definitions
(variable_declaration
  (identifier) @local.definition.var)

(parameter
  name: (identifier) @local.definition.var)

(payload
  (identifier) @local.definition.var)

(function_declaration
  name: (identifier) @local.definition.function)

; References
(identifier) @local.reference
