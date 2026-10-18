package foldervalidator

import (
	"errors"
	"testing"
)

func directory(name string, children ...Node) Node {
	return Node{Name: name, Type: "dir", Children: children}
}

func file(name string) Node {
	return Node{Name: name, Type: "file"}
}

func validInput() FolderValidationInput {
	return FolderValidationInput{
		Expected: directory("project", file("main.go"), directory("internal", file("task.go"))),
		Actual:   directory("project", file("main.go"), directory("internal", file("task.go"))),
	}
}

func TestCreate(t *testing.T) {
	store := NewInMemoryTaskStore(FolderValidator{})
	task, err := store.Create(validInput())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if task.ID == "" || task.State != TaskPending || task.Result != nil {
		t.Fatalf("unexpected created task: %#v", task)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	store := NewInMemoryTaskStore(FolderValidator{})
	_, err := store.Create(FolderValidationInput{Expected: directory("project"), Actual: file("main.go")})
	if !errors.Is(err, ErrInvalidFolderInput) {
		t.Fatalf("Create() error = %v, want ErrInvalidFolderInput", err)
	}
}

func TestGet(t *testing.T) {
	store := NewInMemoryTaskStore(FolderValidator{})
	created, err := store.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	_, err = store.Get("task-404")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("Get(nonexistent) error = %v, want ErrTaskNotFound", err)
	}
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name       string
		input      FolderValidationInput
		wantValid  bool
		violations int
	}{
		{name: "matching trees", input: validInput(), wantValid: true},
		{name: "missing file", input: FolderValidationInput{Expected: directory("project", file("main.go")), Actual: directory("project")}, wantValid: false, violations: 1},
		{name: "slack markdown files", input: FolderValidationInput{Expected: directory("project", file("main.go")), Actual: directory("project", file("main.go"), file("notes.MD")), SlackExtensions: []string{"md"}}, wantValid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewInMemoryTaskStore(FolderValidator{})
			created, err := store.Create(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			task, err := store.Process(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if task.State != TaskCompleted || task.Result == nil || task.Result.Valid != tt.wantValid || len(task.Result.Violations) != tt.violations {
				t.Fatalf("Process() task = %#v", task)
			}
		})
	}
}

func TestProcessRejectsCompletedTask(t *testing.T) {
	store := NewInMemoryTaskStore(FolderValidator{})
	created, err := store.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Process(created.ID); err != nil {
		t.Fatal(err)
	}
	_, err = store.Process(created.ID)
	if !errors.Is(err, ErrInvalidTaskState) {
		t.Fatalf("second Process() error = %v, want ErrInvalidTaskState", err)
	}
}

func TestMultipleIndependentTasks(t *testing.T) {
	store := NewInMemoryTaskStore(FolderValidator{})
	matching, err := store.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}
	mismatchingInput := validInput()
	mismatchingInput.Actual.Children = mismatchingInput.Actual.Children[:1]
	mismatching, err := store.Create(mismatchingInput)
	if err != nil {
		t.Fatal(err)
	}
	if matching.ID == mismatching.ID {
		t.Fatal("task IDs are not unique")
	}
	first, err := store.Process(matching.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Process(mismatching.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Result.Valid || second.Result.Valid {
		t.Fatalf("independent results = %#v, %#v", first.Result, second.Result)
	}
}

func TestValidateRejectsMalformedDomainInput(t *testing.T) {
	tests := []struct {
		name  string
		input FolderValidationInput
	}{
		{
			name:  "file root",
			input: FolderValidationInput{Expected: file("project"), Actual: directory("project")},
		},
		{
			name: "empty child name",
			input: FolderValidationInput{
				Expected: directory("project", file("")),
				Actual:   directory("project"),
			},
		},
		{
			name: "duplicate child names",
			input: FolderValidationInput{
				Expected: directory("project", file("main.go"), file("main.go")),
				Actual:   directory("project"),
			},
		},
		{
			name: "invalid slack extension",
			input: FolderValidationInput{
				Expected:        directory("project"),
				Actual:          directory("project"),
				SlackExtensions: []string{".tar.gz"},
			},
		},
	}

	validator := FolderValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.input)
			if !errors.Is(err, ErrInvalidFolderInput) {
				t.Fatalf("Validate() error = %v, want ErrInvalidFolderInput", err)
			}
		})
	}
}

func TestFolderValidationResultDetailsAndSlack(t *testing.T) {
	tests := []struct {
		name           string
		input          FolderValidationInput
		wantValid      bool
		wantViolations []string
	}{
		{
			name: "reports missing unexpected and wrong type nodes",
			input: FolderValidationInput{
				Expected: directory("project", file("main.go"), directory("config", file("settings.yaml"))),
				Actual:   directory("project", file("README.md"), file("config")),
			},
			wantViolations: []string{
				"project/README.md: unexpected",
				"project/config: expected dir, got file",
				"project/main.go: missing",
			},
		},
		{
			name: "slack files can be absent from actual tree",
			input: FolderValidationInput{
				Expected:        directory("project", file("README.MD"), directory("docs", file("guide.md"))),
				Actual:          directory("project", directory("docs")),
				SlackExtensions: []string{".md"},
			},
			wantValid: true,
		},
	}

	validator := FolderValidator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Process(tt.input)
			if result.Valid != tt.wantValid {
				t.Fatalf("Process().Valid = %t, want %t", result.Valid, tt.wantValid)
			}
			if len(result.Violations) != len(tt.wantViolations) {
				t.Fatalf("Process().Violations = %#v, want %#v", result.Violations, tt.wantViolations)
			}
			for i, want := range tt.wantViolations {
				if result.Violations[i] != want {
					t.Fatalf("Process().Violations[%d] = %q, want %q", i, result.Violations[i], want)
				}
			}
		})
	}
}

func TestProcessNonexistentTask(t *testing.T) {
	store := NewInMemoryTaskStore(FolderValidator{})
	_, err := store.Process("task-404")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("Process(nonexistent) error = %v, want ErrTaskNotFound", err)
	}
}

type blockingProcessor struct {
	started chan struct{}
	release chan struct{}
}

func (p blockingProcessor) Validate(input FolderValidationInput) error {
	return FolderValidator{}.Validate(input)
}

func (p blockingProcessor) Process(input FolderValidationInput) ValidationResult {
	p.started <- struct{}{}
	<-p.release
	return FolderValidator{}.Process(input)
}

func TestProcessTransitionsThroughProcessingAndStoresResult(t *testing.T) {
	processor := blockingProcessor{started: make(chan struct{}, 1), release: make(chan struct{})}
	store := NewInMemoryTaskStore(processor)
	created, err := store.Create(validInput())
	if err != nil {
		t.Fatal(err)
	}

	completed := make(chan error, 1)
	go func() {
		_, err := store.Process(created.ID)
		completed <- err
	}()
	<-processor.started

	processing, err := store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if processing.State != TaskProcessing || processing.Result != nil {
		t.Fatalf("task while processing = %#v", processing)
	}
	if _, err := store.Process(created.ID); !errors.Is(err, ErrInvalidTaskState) {
		t.Fatalf("Process(processing task) error = %v, want ErrInvalidTaskState", err)
	}

	close(processor.release)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	stored, err := store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != TaskCompleted || stored.Result == nil || !stored.Result.Valid {
		t.Fatalf("stored completed task = %#v", stored)
	}
}

func TestStoreReturnsCopiesOfTaskData(t *testing.T) {
	store := NewInMemoryTaskStore(FolderValidator{})
	input := validInput()
	created, err := store.Create(input)
	if err != nil {
		t.Fatal(err)
	}

	input.Expected.Children[0].Name = "changed.go"
	created.Input.Actual.Children[0].Name = "also-changed.go"
	stored, err := store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Input.Expected.Children[0].Name != "main.go" || stored.Input.Actual.Children[0].Name != "main.go" {
		t.Fatalf("stored input was changed through caller-owned data: %#v", stored.Input)
	}
}
