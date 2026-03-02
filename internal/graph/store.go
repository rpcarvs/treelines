package graph

import "lines/internal/model"

// GraphStore defines the contract for graph database operations on code elements and edges.
type GraphStore interface {
	Open(path string) error
	Close() error
	CreateSchema() error
	UpsertElement(el model.Element) error
	UpsertEdge(e model.Edge) error
	DeleteElement(id string) error
	DeleteEdgesForFile(path string) error
	DeleteElementsByFile(path string) error
	GetElement(fqName string) (*model.Element, error)
	GetElementByExactName(name string) ([]model.Element, error)
	GetElementsByName(name string) ([]model.Element, error)
	GetCallers(fqName string) ([]model.Element, error)
	GetCallees(fqName string) ([]model.Element, error)
	GetContained(name string) ([]model.Element, error)
	Search(substring string) ([]model.Element, error)
	GetAllElements() ([]model.Element, error)
	DeleteEdgesByType(edgeType string) error
	RunSQL(query string) ([]map[string]any, error)
}
