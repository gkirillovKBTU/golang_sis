# Folder validation

## Domain

folder-structure validation as a small task.
The command receives an expected directory tree from a YAML file and builds the actual tree by recursively inspecting a target folder. Internally, each validation is represented as a task and both trees use `Node` values.
node has a name, a type (dir or file), and optional children 
the comparison produces a ValidationResult: either a valid result or a sorted list of missing, unexpected, wrongly typed, or wrongly named nodes

slack mode: files with a slack extension are ignored anywhere in either tree, so their presence or absence does not create a violation

## Task

Tasks move through this synchronous lifecycle:

```text
pending -> processing -> completed
```

- `pending`: the task was created after its input was validated and has no result
- `processing`: the validator is comparing the two trees
- `completed`: processing has finished, and the task stores its `ValidationResult`

## CLI usage

Run the supplied implementation's workflow from the module root:

```bash
go run ./cmd/folder-validator \
  -dir_struct path/to/expected.yaml \
  -test_folder path/to/folder
```

The two path arguments are required. The command prints a success message and exits `0` for a match, prints sorted violations and exits `1` for a mismatch, uses `2` for invalid arguments/input, and uses `3` for filesystem or processing failures

Slack mode ignores files with the selected extension anywhere in either tree:

```bash
go run ./cmd/folder-validator \
  -dir_struct path/to/expected.yaml \
  -test_folder path/to/folder \
  -slack -slack_format md
```

## Library usage

```go
store := NewInMemoryTaskStore(FolderValidator{})
input := FolderValidationInput{
    Expected: Node{Name: "project", Type: "dir", Children: []Node{{Name: "main.go", Type: "file"}}},
    Actual:   Node{Name: "project", Type: "dir", Children: []Node{{Name: "main.go", Type: "file"}}},
}

task, err := store.Create(input)
if err != nil {
    // handle invalid input
}
completed, err := store.Process(task.ID)
if err != nil {
    // handle a missing task or invalid transition
}
fmt.Println(completed.Result.Valid) // true
```

## Running checks

This project requires Go 1.27. From the module root, run:

```bash
gofmt -w .
go vet ./...
go test ./...
```
