package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func printDiff(out io.Writer, label string, oldBody string, newBody string) {
	if oldBody == newBody {
		fmt.Fprintf(out, "No changes in %s\n", label)
		return
	}
	patch, err := unifiedDiff(label, oldBody, newBody)
	if err != nil {
		fmt.Fprintf(out, "Failed to generate diff for %s: %v\n", label, err)
		return
	}
	fmt.Fprintln(out, patch)
}

func unifiedDiff(label string, oldBody string, newBody string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gitlab-ci-helper-diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	oldPath := filepath.Join(tmpDir, "old")
	newPath := filepath.Join(tmpDir, "new")
	if err := os.WriteFile(oldPath, []byte(oldBody), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(newPath, []byte(newBody), 0o644); err != nil {
		return "", err
	}

	cmd := exec.Command("diff", "-u", "--label", "a/"+label, "--label", "b/"+label, oldPath, newPath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return string(output), nil
	}
	return "", err
}
