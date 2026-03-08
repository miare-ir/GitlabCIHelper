package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

// ApplyPlannedChange writes a previously planned setup update.
func ApplyPlannedChange(cwd string, planned PlannedSetupChange) error {
	if err := writeRepoFileAtomic(cwd, planned.RootPipeline.RelativePath, planned.RootPipeline.UpdatedBody, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", planned.RootPipeline.RelativePath, err)
	}
	if err := writeRepoFileAtomic(cwd, planned.Config.RelativePath, planned.Config.UpdatedBody, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", planned.Config.RelativePath, err)
	}
	for _, asset := range planned.Assets {
		if err := writeRepoFileAtomic(cwd, asset.RelativePath, asset.Body, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", asset.RelativePath, err)
		}
	}
	return nil
}

func writeRepoFileAtomic(cwd, relativePath string, body []byte, fallback os.FileMode) error {
	targetPath := filepath.Join(cwd, relativePath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(relativePath), err)
	}
	perm := existingPerm(targetPath, fallback)
	return writeFileAtomic(targetPath, body, perm)
}

func existingPerm(path string, fallback os.FileMode) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return fallback
	}
	return info.Mode().Perm()
}

func writeFileAtomic(path string, body []byte, perm os.FileMode) (retErr error) {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".gitlab-ci-helper-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if retErr != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(body); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}
