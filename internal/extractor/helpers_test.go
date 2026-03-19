package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rpcarvs/treelines/internal/model"
	"github.com/rpcarvs/treelines/internal/parser"
)

func TestExtractCallQualifier_RustScopedIdentifier(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mod.rs")
	source := []byte("fn process() { crate::camera::start_camera(); }\n")
	if err := osWriteFile(path, source); err != nil {
		t.Fatalf("write rust file: %v", err)
	}

	p := parser.NewParser()
	defer p.Close()
	result, err := p.ParseFile(path, "mod.rs", model.LangRust)
	if err != nil {
		t.Fatalf("parse rust file: %v", err)
	}
	defer result.Tree.Close()

	queryStr, err := loadQuery(model.LangRust)
	if err != nil {
		t.Fatalf("load query: %v", err)
	}
	matches, names, err := runQuery(queryStr, getLanguage(model.LangRust), result.Tree.RootNode(), result.Source)
	if err != nil {
		t.Fatalf("run query: %v", err)
	}

	found := false
	for _, m := range matches {
		caps := captureMap(m, names)
		callNameNode, ok := caps["call_name"]
		if !ok || callNameNode == nil {
			continue
		}
		if nodeText(callNameNode, result.Source) != "start_camera" {
			continue
		}
		found = true
		got := extractCallQualifier(callNameNode, result.Source)
		if got != "crate::camera" {
			t.Fatalf("expected qualifier crate::camera, got %q", got)
		}
	}
	if !found {
		t.Fatalf("expected to capture start_camera call")
	}
}

func TestExpandRustUsePaths(t *testing.T) {
	got := expandRustUsePaths("use crate::ml::{segmentation::detect, recognition};")
	want := map[string]struct{}{
		"crate::ml::segmentation::detect": {},
		"crate::ml::recognition":          {},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d paths, got %d: %v", len(want), len(got), got)
	}
	for _, path := range got {
		if _, ok := want[path]; !ok {
			t.Fatalf("unexpected path: %s", path)
		}
	}
}

func TestGoImportToDir(t *testing.T) {
	modulePath := "github.com/rpcarvs/treelines"
	if got := goImportToDir("github.com/rpcarvs/treelines/internal/scanner", modulePath); got != "internal/scanner" {
		t.Fatalf("expected internal/scanner, got %q", got)
	}
	if got := goImportToDir("github.com/x/y", modulePath); got != "" {
		t.Fatalf("expected empty path for external import, got %q", got)
	}
}

func TestParseGoImportSpecs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	source := []byte(`package main
import (
	cam "github.com/rpcarvs/treelines/internal/camera"
	"github.com/rpcarvs/treelines/internal/ml"
)
func main() { cam.Start(); ml.Run() }`)
	if err := osWriteFile(path, source); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	p := parser.NewParser()
	defer p.Close()
	result, err := p.ParseFile(path, "main.go", model.LangGo)
	if err != nil {
		t.Fatalf("parse go file: %v", err)
	}
	defer result.Tree.Close()

	queryStr, err := loadQuery(model.LangGo)
	if err != nil {
		t.Fatalf("load query: %v", err)
	}
	matches, names, err := runQuery(queryStr, getLanguage(model.LangGo), result.Tree.RootNode(), result.Source)
	if err != nil {
		t.Fatalf("run query: %v", err)
	}

	specs := parseGoImportSpecs(matches, names, result.Source)
	want := map[string]string{
		"cam": "github.com/rpcarvs/treelines/internal/camera",
		"ml":  "github.com/rpcarvs/treelines/internal/ml",
	}
	if len(specs) != len(want) {
		t.Fatalf("expected %d specs, got %d: %+v", len(want), len(specs), specs)
	}
	for _, spec := range specs {
		expectedPath, ok := want[spec.Binding]
		if !ok {
			t.Fatalf("unexpected binding %q", spec.Binding)
		}
		if spec.Path != expectedPath {
			t.Fatalf("binding %q expected path %q, got %q", spec.Binding, expectedPath, spec.Path)
		}
	}
}

func TestExtractRustCallImportMaps(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mod.rs")
	source := []byte(`use crate::camera;
use crate::ml::segmentation as seg;
use crate::image_utils::preprocess_for_yolo;
mod recognition;
fn process() { camera::start_camera(); seg::detect(); }`)
	if err := osWriteFile(path, source); err != nil {
		t.Fatalf("write rust file: %v", err)
	}

	p := parser.NewParser()
	defer p.Close()
	result, err := p.ParseFile(path, "mod.rs", model.LangRust)
	if err != nil {
		t.Fatalf("parse rust file: %v", err)
	}
	defer result.Tree.Close()

	queryStr, err := loadQuery(model.LangRust)
	if err != nil {
		t.Fatalf("load query: %v", err)
	}
	matches, names, err := runQuery(queryStr, getLanguage(model.LangRust), result.Tree.RootNode(), result.Source)
	if err != nil {
		t.Fatalf("run query: %v", err)
	}

	all := []model.Element{
		{ID: "m1", Language: model.LangRust, Kind: model.KindModule, FQName: "crate::camera"},
		{ID: "m2", Language: model.LangRust, Kind: model.KindModule, FQName: "crate::ml::segmentation"},
		{ID: "f1", Language: model.LangRust, Kind: model.KindFunction, FQName: "crate::image_utils::preprocess_for_yolo"},
	}
	resolver := NewResolver(all)
	allByID := buildElementsByID(all)
	maps := extractRustCallImportMaps(matches, names, result.Source, "crate::ml", resolver, allByID)
	if maps == nil {
		t.Fatalf("expected rust import maps")
	}
	if maps.qualifierByName["camera"] != "crate::camera" {
		t.Fatalf("camera alias mismatch: %q", maps.qualifierByName["camera"])
	}
	if maps.qualifierByName["seg"] != "crate::ml::segmentation" {
		t.Fatalf("seg alias mismatch: %q", maps.qualifierByName["seg"])
	}
	if maps.qualifierByName["recognition"] != "crate::ml::recognition" {
		t.Fatalf("recognition module alias mismatch: %q", maps.qualifierByName["recognition"])
	}
	if maps.symbolByName["preprocess_for_yolo"] != "crate::image_utils::preprocess_for_yolo" {
		t.Fatalf("symbol alias mismatch: %q", maps.symbolByName["preprocess_for_yolo"])
	}
}

func osWriteFile(path string, data []byte) error { return os.WriteFile(path, data, 0o644) }
