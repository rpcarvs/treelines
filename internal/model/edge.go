package model

const (
	EdgeCalls      = "CALLS"
	EdgeImports    = "IMPORTS"
	EdgeContains   = "CONTAINS"
	EdgeImplements = "IMPLEMENTS"
	EdgeExtends    = "EXTENDS"
	EdgeReferences = "REFERENCES"
	EdgeDefinedIn  = "DEFINED_IN"
)

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}
