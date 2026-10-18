package foldervalidator

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrInvalidTaskState   = errors.New("invalid task state")
	ErrInvalidFolderInput = errors.New("invalid folder validation input")
)

type TaskState string

const (
	TaskPending    TaskState = "pending"
	TaskProcessing TaskState = "processing"
	TaskCompleted  TaskState = "completed"
)

type Node struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Children []Node `yaml:"children,omitempty"`
}

type FolderValidationInput struct {
	Expected        Node
	Actual          Node
	SlackExtensions []string
}

type ValidationResult struct {
	Valid      bool
	Violations []string
}

type Task struct {
	ID     string
	Input  FolderValidationInput
	State  TaskState
	Result *ValidationResult
}

type FolderProcessor interface {
	Validate(FolderValidationInput) error
	Process(FolderValidationInput) ValidationResult
}

type TaskStore interface {
	Create(FolderValidationInput) (Task, error)
	Get(string) (Task, error)
	Process(string) (Task, error)
}

type FolderValidator struct{}

func (FolderValidator) Validate(input FolderValidationInput) error {
	if err := validateRoot(input.Expected, "expected"); err != nil {
		return err
	}
	if err := validateRoot(input.Actual, "actual"); err != nil {
		return err
	}
	for _, extension := range input.SlackExtensions {
		if normalizeExtension(extension) == "" {
			return fmt.Errorf("%w: slack extension %q is invalid", ErrInvalidFolderInput, extension)
		}
	}
	return nil
}

func (FolderValidator) Process(input FolderValidationInput) ValidationResult {
	ignored := make(map[string]struct{}, len(input.SlackExtensions))
	for _, extension := range input.SlackExtensions {
		ignored[normalizeExtension(extension)] = struct{}{}
	}

	violations := compareNodes(input.Expected, input.Actual, input.Expected.Name, ignored)
	sort.Strings(violations)
	return ValidationResult{Valid: len(violations) == 0, Violations: violations}
}

type InMemoryTaskStore struct {
	tasks     map[string]Task
	nextID    uint64
	processor FolderProcessor
}

func NewInMemoryTaskStore(processor FolderProcessor) *InMemoryTaskStore {
	if processor == nil {
		processor = FolderValidator{}
	}
	return &InMemoryTaskStore{tasks: make(map[string]Task), processor: processor}
}

func (s *InMemoryTaskStore) Create(input FolderValidationInput) (Task, error) {
	if err := s.processor.Validate(input); err != nil {
		return Task{}, err
	}

	s.nextID++
	task := Task{
		ID:    fmt.Sprintf("task-%d", s.nextID),
		Input: cloneInput(input),
		State: TaskPending,
	}
	s.tasks[task.ID] = task
	return cloneTask(task), nil
}

func (s *InMemoryTaskStore) Get(id string) (Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}
	return cloneTask(task), nil
}

func (s *InMemoryTaskStore) Process(id string) (Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	}
	if task.State != TaskPending {
		return Task{}, fmt.Errorf("%w: cannot process task in %s state", ErrInvalidTaskState, task.State)
	}

	task.State = TaskProcessing
	s.tasks[id] = task
	result := s.processor.Process(task.Input)
	task.Result = &result
	task.State = TaskCompleted
	s.tasks[id] = task
	return cloneTask(task), nil
}

func validateRoot(node Node, label string) error {
	if node.Type != "dir" {
		return fmt.Errorf("%w: %s root must be a directory", ErrInvalidFolderInput, label)
	}
	return validateNode(node, label)
}

func validateNode(node Node, path string) error {
	if strings.TrimSpace(node.Name) == "" || strings.ContainsAny(node.Name, `/\\`) {
		return fmt.Errorf("%w: invalid node name at %s", ErrInvalidFolderInput, path)
	}
	if node.Type != "file" && node.Type != "dir" {
		return fmt.Errorf("%w: invalid node type at %s", ErrInvalidFolderInput, path)
	}
	if node.Type == "file" && len(node.Children) > 0 {
		return fmt.Errorf("%w: file has children at %s", ErrInvalidFolderInput, path)
	}
	seen := make(map[string]struct{}, len(node.Children))
	for _, child := range node.Children {
		if _, exists := seen[child.Name]; exists {
			return fmt.Errorf("%w: duplicate child %q at %s", ErrInvalidFolderInput, child.Name, path)
		}
		seen[child.Name] = struct{}{}
		if err := validateNode(child, joinPath(path, child.Name)); err != nil {
			return err
		}
	}
	return nil
}

func compareNodes(expected, actual Node, path string, ignored map[string]struct{}) []string {
	violations := make([]string, 0)
	if expected.Name != actual.Name {
		violations = append(violations, fmt.Sprintf("%s: expected name %q, got %q", path, expected.Name, actual.Name))
	}
	if expected.Type != actual.Type {
		return append(violations, fmt.Sprintf("%s: expected %s, got %s", path, expected.Type, actual.Type))
	}
	if expected.Type == "file" {
		return violations
	}

	expectedChildren := childrenByName(expected.Children, ignored)
	actualChildren := childrenByName(actual.Children, ignored)
	for name, expectedChild := range expectedChildren {
		actualChild, exists := actualChildren[name]
		childPath := joinPath(path, name)
		if !exists {
			violations = append(violations, fmt.Sprintf("%s: missing", childPath))
			continue
		}
		violations = append(violations, compareNodes(expectedChild, actualChild, childPath, ignored)...)
	}
	for name := range actualChildren {
		if _, exists := expectedChildren[name]; !exists {
			violations = append(violations, fmt.Sprintf("%s: unexpected", joinPath(path, name)))
		}
	}
	return violations
}

func childrenByName(children []Node, ignored map[string]struct{}) map[string]Node {
	result := make(map[string]Node, len(children))
	for _, child := range children {
		if child.Type == "file" {
			if _, skip := ignored[strings.ToLower(filepath.Ext(child.Name))]; skip {
				continue
			}
		}
		result[child.Name] = child
	}
	return result
}

func normalizeExtension(extension string) string {
	extension = strings.TrimSpace(strings.ToLower(extension))
	extension = strings.TrimPrefix(extension, ".")
	if extension == "" || strings.ContainsAny(extension, `/\\.`) {
		return ""
	}
	return "." + extension
}

func joinPath(parent, name string) string {
	return parent + "/" + name
}

func cloneInput(input FolderValidationInput) FolderValidationInput {
	return FolderValidationInput{
		Expected:        cloneNode(input.Expected),
		Actual:          cloneNode(input.Actual),
		SlackExtensions: append([]string(nil), input.SlackExtensions...),
	}
}

func cloneTask(task Task) Task {
	task.Input = cloneInput(task.Input)
	if task.Result != nil {
		result := ValidationResult{Valid: task.Result.Valid, Violations: append([]string(nil), task.Result.Violations...)}
		task.Result = &result
	}
	return task
}

func cloneNode(node Node) Node {
	copy := Node{Name: node.Name, Type: node.Type}
	if len(node.Children) > 0 {
		copy.Children = make([]Node, len(node.Children))
		for i := range node.Children {
			copy.Children[i] = cloneNode(node.Children[i])
		}
	}
	return copy
}
