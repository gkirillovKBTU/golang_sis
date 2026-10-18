package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	foldervalidator "folder_validator"
)

const (
	exitValid     = 0
	exitInvalid   = 1
	exitUsage     = 2
	exitExecution = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("folder-validator", flag.ContinueOnError)
	flags.SetOutput(stderr)
	structurePath := flags.String("dir_struct", "", "path to the YAML file describing the expected folder structure")
	targetPath := flags.String("test_folder", "", "path to the folder to validate")
	slack := flags.Bool("slack", false, "ignore files with the configured extension anywhere in the tree")
	slackFormat := flags.String("slack_format", "md", "extension ignored in slack mode (for example: md)")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: folder-validator -dir_struct expected.yaml -test_folder path [-slack] [-slack_format md]\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "error: unexpected positional arguments: %s\n", strings.Join(flags.Args(), " "))
		flags.Usage()
		return exitUsage
	}
	if strings.TrimSpace(*structurePath) == "" || strings.TrimSpace(*targetPath) == "" {
		fmt.Fprintln(stderr, "error: both -dir_struct and -test_folder are required")
		flags.Usage()
		return exitUsage
	}

	expected, err := foldervalidator.LoadExpectedTree(*structurePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitExecution
	}
	actual, err := foldervalidator.BuildTree(*targetPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitExecution
	}

	input := foldervalidator.FolderValidationInput{Expected: expected, Actual: actual}
	if *slack {
		input.SlackExtensions = []string{*slackFormat}
	}
	store := foldervalidator.NewInMemoryTaskStore(foldervalidator.FolderValidator{})
	task, err := store.Create(input)
	if err != nil {
		if errors.Is(err, foldervalidator.ErrInvalidFolderInput) {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return exitUsage
		}
		fmt.Fprintf(stderr, "error: create validation task: %v\n", err)
		return exitExecution
	}
	completed, err := store.Process(task.ID)
	if err != nil {
		fmt.Fprintf(stderr, "error: process validation task: %v\n", err)
		return exitExecution
	}
	if completed.Result.Valid {
		fmt.Fprintln(stdout, "directory structure matches")
		return exitValid
	}

	fmt.Fprintln(stdout, "directory structure does NOT match")
	for _, violation := range completed.Result.Violations {
		fmt.Fprintf(stdout, "- %s\n", violation)
	}
	return exitInvalid
}
