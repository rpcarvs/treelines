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
(import_statement) @import

(import_from_statement) @import

;; Static export surface
(assignment
  left: (identifier) @assign_name
  right: (_) @assign_value
) @assignment

;; Call expressions
(call
  function: (identifier) @call_name
) @call

(call
  function: (attribute
    attribute: (identifier) @call_name
  )
) @call
