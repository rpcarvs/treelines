;; Functions (top-level and nested)
(function_definition
  name: (identifier) @name
) @element

;; Classes
(class_definition
  name: (identifier) @name
  superclasses: (argument_list)? @bases
) @element

;; Decorated definitions
(decorated_definition
  definition: (function_definition
    name: (identifier) @name
  ) @element
)

(decorated_definition
  definition: (class_definition
    name: (identifier) @name
  ) @element
)

;; Import statements
(import_statement
  name: (dotted_name) @import_name
) @import

(import_from_statement
  module_name: (dotted_name) @module
  name: (dotted_name) @import_name
) @import

;; Call expressions
(call
  function: (identifier) @call_name
) @call

(call
  function: (attribute
    attribute: (identifier) @call_name
  )
) @call
