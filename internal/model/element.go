package model

import (
	"crypto/sha256"
	"fmt"
)

const (
	KindFunction  = "function"
	KindMethod    = "method"
	KindClass     = "class"
	KindStruct    = "struct"
	KindInterface = "interface"
	KindTrait     = "trait"
	KindEnum      = "enum"
	KindImpl      = "impl"
	KindModule    = "module"
)

const (
	LangPython = "python"
	LangGo     = "go"
	LangRust   = "rust"
)

const (
	VisPublic  = "public"
	VisPrivate = "private"
)

type Element struct {
	ID         string `json:"id"`
	Language   string `json:"language"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	FQName     string `json:"fq_name"`
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	LOC        int    `json:"loc"`
	Signature  string `json:"signature"`
	Visibility string `json:"visibility"`
	Docstring  string `json:"docstring"`
}

// MakeID generates a deterministic ID from language, path, and fully qualified name.
func MakeID(lang, path, fqName string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", lang, path, fqName)))
	return fmt.Sprintf("%x", h[:12])
}
