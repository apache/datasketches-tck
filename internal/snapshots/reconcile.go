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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Result describes the generated revision and its differences from the existing
// snapshot directory, including differences skipped by the update policy.
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

// Check regenerates a source at its configured commit without writing repository
// files. Snapshot differences are returned in Result rather than as errors.
func Check(ctx context.Context, root, language string, stdout, stderr io.Writer) (Result, error) {
	config, _, err := loadConfig(root)
	if err != nil {
		return Result{}, err
	}
	source, found := config.source(language)
	if !found {
		return Result{}, fmt.Errorf("unsupported snapshot language %q", language)
	}
	workspace, err := os.MkdirTemp("", "datasketches-tck-check-")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	return generate(ctx, root, workspace, language, source, source.Commit, commandRunner{stdout: stdout, stderr: stderr})
}

// Update generates every selected source before publishing any changes. Empty
// language selects all sources; empty revision selects each remote's HEAD.
// With skipUnstableOnly, only additions, deletions, or stable content changes
// cause a source's snapshots and pin to be adopted; otherwise both are retained.
func Update(ctx context.Context, root, language, revision string, skipUnstableOnly bool, stdout, stderr io.Writer) ([]Result, error) {
	languages := Languages()
	if language != "" {
		languages = []string{language}
	}
	if revision == "" {
		revision = "HEAD"
	}
	config, originalConfig, err := loadConfig(root)
	if err != nil {
		return nil, err
	}
	workspace, err := os.MkdirTemp("", "datasketches-tck-update-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(workspace) }()

	var results []Result
	var replacements []replacement
	revisionChanged := false
	for _, language := range languages {
		source, found := config.source(language)
		if !found {
			return nil, fmt.Errorf("unsupported snapshot language %q", language)
		}
		directory := filepath.Join(workspace, language)
		result, err := generate(ctx, root, directory, language, source, revision, commandRunner{stdout: stdout, stderr: stderr})
		if err != nil {
			return nil, fmt.Errorf("update aborted before publishing: %w", err)
		}
		results = append(results, result)
		if skipUnstableOnly && !result.HasBlockingChanges() {
			continue
		}
		if result.RevisionChanged() {
			if err := config.setCommit(language, result.Revision); err != nil {
				return nil, err
			}
			revisionChanged = true
		}
		if len(result.Changes) > 0 {
			replacements = append(replacements, replacement{
				target: result.Target,
				source: filepath.Join(directory, "generated"),
			})
		}
	}
	if revisionChanged {
		content, err := encodeConfig(config)
		if err != nil {
			return nil, err
		}
		nextConfig := filepath.Join(workspace, configFilename)
		if err := os.WriteFile(nextConfig, content, 0o644); err != nil {
			return nil, err
		}
		replacements = append(replacements, replacement{target: filepath.Join(root, configFilename), source: nextConfig})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A long-running generation must not overwrite a pin edited in the meantime.
	currentConfig, err := os.ReadFile(filepath.Join(root, configFilename))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(currentConfig, originalConfig) {
		return nil, fmt.Errorf("config.toml changed during generation; no updates were published")
	}
	if err := publish(root, replacements, stderr); err != nil {
		return nil, err
	}
	return results, nil
}

func generate(ctx context.Context, root, workspace, language string, source snapshotSource, revision string, runner commandRunner) (Result, error) {
	definition, found := generators[language]
	if !found {
		return Result{}, fmt.Errorf("unsupported snapshot language %q", language)
	}
	for _, requirement := range definition.requirements {
		if _, err := exec.LookPath(requirement); err != nil {
			return Result{}, fmt.Errorf("required command %q is not installed or not on PATH", requirement)
		}
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return Result{}, err
	}
	if _, err := fmt.Fprintf(runner.stdout, "Generate %s snapshots\n", language); err != nil {
		return Result{}, err
	}
	generated := filepath.Join(workspace, "generated")
	resolvedRevision, err := definition.run(ctx, workspace, generated, source.Repository, revision, runner)
	if err != nil {
		return Result{}, fmt.Errorf("generate %s snapshots: %w", language, err)
	}
	target := snapshotDirectory(root, language)
	changes, err := compareDirectories(target, generated, definition.stability)
	if err != nil {
		return Result{}, err
	}
	return Result{Target: target, PreviousRevision: source.Commit, Revision: resolvedRevision, Changes: changes}, nil
}

func snapshotDirectory(root, language string) string {
	return filepath.Join(root, "serialization_test_data", language+"_generated_files")
}
