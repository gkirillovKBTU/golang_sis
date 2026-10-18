package foldervalidator

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

func LoadExpectedTree(path string) (Node, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Node{}, fmt.Errorf("inspect structure file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Node{}, fmt.Errorf("structure path %q is not a regular file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Node{}, fmt.Errorf("read structure file %q: %w", path, err)
	}

	var root Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return Node{}, fmt.Errorf("parse structure file %q: %w", path, err)
	}
	if err := validateRoot(root, "expected"); err != nil {
		return Node{}, err
	}
	return root, nil
}

func BuildTree(path string) (Node, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Node{}, fmt.Errorf("inspect target path %q: %w", path, err)
	}

	node := Node{Name: info.Name()}
	if !info.IsDir() {
		node.Type = "file"
		return node, nil
	}

	node.Type = "dir"
	entries, err := os.ReadDir(path)
	if err != nil {
		return Node{}, fmt.Errorf("read target directory %q: %w", path, err)
	}
	for _, entry := range entries {
		child, err := BuildTree(filepath.Join(path, entry.Name()))
		if err != nil {
			return Node{}, err
		}
		node.Children = append(node.Children, child)
	}
	return node, nil
}
