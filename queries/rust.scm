;; Function items
(function_item
  name: (identifier) @name
) @element

;; Struct items
(struct_item
  name: (type_identifier) @name
) @element

;; Enum items
(enum_item
  name: (type_identifier) @name
) @element

;; Trait items
(trait_item
  name: (type_identifier) @name
) @element

;; Impl items
(impl_item
  trait: (type_identifier)? @trait_name
  type: (type_identifier) @name
) @element

;; Use declarations
(use_declaration
  argument: (_) @use_path
) @import

;; Call expressions
(call_expression
  function: (identifier) @call_name
) @call

(call_expression
  function: (field_expression
    field: (field_identifier) @call_name
  )
) @call

(call_expression
  function: (scoped_identifier
    name: (identifier) @call_name
  )
) @call
