/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package snapshots

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Mode string

const (
	ModeCheck  Mode = "check"
	ModeUpdate Mode = "update"
)

type Result struct {
	Target           string
	PreviousRevision string
	Revision         string
	Changes          []Change
}

func (result Result) RevisionChanged() bool {
	return result.PreviousRevision != result.Revision
}

func (result Result) HasBlockingChanges() bool {
	return result.BlockingChangeCount() > 0
}

func (result Result) BlockingChangeCount() int {
	count := 0
	for _, change := range result.Changes {
		if change.IsBlocking() {
			count++
		}
	}
	return count
}

func Reconcile(
	ctx context.Context,
	repositoryRoot, language string,
	mode Mode,
	requestedRevision string,
	stdout, stderr io.Writer,
) (Result, error) {
	if mode != ModeCheck && mode != ModeUpdate {
		return Result{}, fmt.Errorf("unsupported reconciliation mode %q", mode)
	}
	generator, found := generators[language]
	if !found {
		return Result{}, fmt.Errorf("unsupported snapshot language %q", language)
	}
	if mode != ModeUpdate && requestedRevision != "" {
		return Result{}, fmt.Errorf("a source revision can only be selected in update mode")
	}
	revisions, originalRevisions, err := loadRevisionConfig(repositoryRoot)
	if err != nil {
		return Result{}, err
	}
	pinnedRevision, found := revisions.revision(language)
	if !found {
		return Result{}, fmt.Errorf("snapshot revision for unsupported language %q", language)
	}
	sourceRevision := pinnedRevision
	if requestedRevision != "" {
		sourceRevision = requestedRevision
	}
	for _, requirement := range generator.requirements {
		if _, err := exec.LookPath(requirement); err != nil {
			return Result{}, fmt.Errorf("required command %q is not installed or not on PATH", requirement)
		}
	}

	workspace, err := os.MkdirTemp("", "datasketches-tck-"+language+"-")
	if err != nil {
		return Result{}, fmt.Errorf("create generator workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(workspace) }()

	generated := filepath.Join(workspace, "generated")
	runner := commandRunner{stdout: stdout, stderr: stderr}
	resolvedRevision, err := generator.run(ctx, workspace, generated, sourceRevision, runner)
	if err != nil {
		return Result{}, fmt.Errorf("generate %s snapshots: %w", language, err)
	}

	target := filepath.Join(repositoryRoot, "serialization", language, "snapshots")
	changes, err := compareDirectories(target, generated, generator.stability)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Target:           target,
		PreviousRevision: pinnedRevision,
		Revision:         resolvedRevision,
		Changes:          changes,
	}
	if mode != ModeUpdate {
		return result, nil
	}

	revisionChanged := result.RevisionChanged()
	if revisionChanged {
		if err := revisions.setRevision(language, resolvedRevision); err != nil {
			return Result{}, err
		}
		updatedRevisions, err := encodeRevisionConfig(revisions)
		if err != nil {
			return Result{}, err
		}
		if err := replaceRevisionFile(repositoryRoot, updatedRevisions); err != nil {
			return Result{}, err
		}
	}
	if len(changes) > 0 {
		if err := replaceDirectory(target, generated); err != nil {
			if !revisionChanged {
				return Result{}, err
			}
			if restoreErr := replaceRevisionFile(repositoryRoot, originalRevisions); restoreErr != nil {
				return Result{}, fmt.Errorf(
					"%w; restoring previous snapshot revisions also failed: %v",
					err,
					restoreErr,
				)
			}
			return Result{}, err
		}
	}
	return result, nil
}

func replaceDirectory(target, generated string) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create snapshot parent directory: %w", err)
	}

	transaction, err := os.MkdirTemp(parent, "."+filepath.Base(target)+"-tck-")
	if err != nil {
		return fmt.Errorf("create snapshot update transaction: %w", err)
	}
	removeTransaction := true
	defer func() {
		if removeTransaction {
			_ = os.RemoveAll(transaction)
		}
	}()

	next := filepath.Join(transaction, "next")
	if err := copyDirectory(generated, next); err != nil {
		return fmt.Errorf("stage generated snapshots: %w", err)
	}

	previous := filepath.Join(transaction, "previous")
	hadTarget := false
	info, err := os.Lstat(target)
	switch {
	case err == nil:
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("replace snapshots: %s is not a directory", target)
		}
		hadTarget = true
		if err := os.Rename(target, previous); err != nil {
			return fmt.Errorf("preserve current snapshots: %w", err)
		}
	case os.IsNotExist(err):
	case err != nil:
		return fmt.Errorf("inspect snapshots before update: %w", err)
	}

	if err := os.Rename(next, target); err != nil {
		if !hadTarget {
			return fmt.Errorf("install generated snapshots: %w", err)
		}
		if restoreErr := os.Rename(previous, target); restoreErr == nil {
			return fmt.Errorf("install generated snapshots: %w", err)
		} else {
			removeTransaction = false
			return fmt.Errorf(
				"install generated snapshots: %w; restoring previous snapshots also failed: %v; previous files remain at %s",
				err,
				restoreErr,
				previous,
			)
		}
	}

	if hadTarget {
		if err := os.RemoveAll(previous); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove previous snapshots after update: %w", err)
		}
	}
	return nil
}

func copyDirectory(source, destination string) error {
	files, err := regularFiles(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	for relative, file := range files {
		output := filepath.Join(destination, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		if err := copyFile(file.filename, output); err != nil {
			return err
		}
	}
	return nil
}
