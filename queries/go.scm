;; Function declarations
(function_declaration
  name: (identifier) @name
) @element

;; Method declarations (with receiver)
(method_declaration
  receiver: (parameter_list) @receiver
  name: (field_identifier) @name
) @element

;; Type declarations - struct
(type_declaration
  (type_spec
    name: (type_identifier) @name
    type: (struct_type)
  )
) @element

;; Type declarations - interface
(type_declaration
  (type_spec
    name: (type_identifier) @name
    type: (interface_type)
  )
) @element

;; Import specs
(import_spec
  path: (interpreted_string_literal) @import_path
) @import

;; Call expressions
(call_expression
  function: (identifier) @call_name
) @call

(call_expression
  function: (selector_expression
    field: (field_identifier) @call_name
  )
) @call
