package foldervalidator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExpectedTreeAndBuildTreeUseRealPaths(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(target, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(target, "main.go"), filepath.Join(target, "nested", "task.go")} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	structure := filepath.Join(root, "expected.yaml")
	yaml := "name: project\ntype: dir\nchildren:\n  - name: main.go\n    type: file\n  - name: nested\n    type: dir\n    children:\n      - name: task.go\n        type: file\n"
	if err := os.WriteFile(structure, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	expected, err := LoadExpectedTree(structure)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := BuildTree(target)
	if err != nil {
		t.Fatal(err)
	}
	result := (FolderValidator{}).Process(FolderValidationInput{Expected: expected, Actual: actual})
	if !result.Valid {
		t.Fatalf("filesystem tree should match YAML: %#v", result.Violations)
	}
}

func TestFilesystemInputsReportBadPaths(t *testing.T) {
	if _, err := LoadExpectedTree(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("LoadExpectedTree() accepted a missing file")
	}
	if _, err := BuildTree(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("BuildTree() accepted a missing target directory")
	}
}
