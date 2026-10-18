package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequiresCLIPaths(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != exitUsage {
		t.Fatalf("run() exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "-dir_struct") || !strings.Contains(stderr.String(), "-test_folder") {
		t.Fatalf("usage does not document required paths: %q", stderr.String())
	}
}

func TestRunValidatesFolderFromCLIArguments(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "project")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	structure := filepath.Join(root, "expected.yaml")
	if err := os.WriteFile(structure, []byte("name: project\ntype: dir\nchildren:\n  - name: main.go\n    type: file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir_struct", structure, "-test_folder", target}, &stdout, &stderr)
	if code != exitValid || !strings.Contains(stdout.String(), "matches") || stderr.Len() != 0 {
		t.Fatalf("run() = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
