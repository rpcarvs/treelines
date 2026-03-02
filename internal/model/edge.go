package model

const (
	EdgeCalls      = "CALLS"
	EdgeContains   = "CONTAINS"
	EdgeImplements = "IMPLEMENTS"
	EdgeExtends    = "EXTENDS"
	EdgeDefinedIn  = "DEFINED_IN"
)

// Edge represents a directed relationship between two elements.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}
