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
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateGeneratesEntireBatchBeforePublishing(t *testing.T) {
	root, oldRevision, newRevision := updateFixture(t)
	calls := 0
	for language, definition := range generators {
		definition.generate = func(_ context.Context, paths generationPaths, _ commandRunner) error {
			config, _, err := loadConfig(root)
			require.NoError(t, err)
			for _, selected := range Languages() {
				source, _ := config.source(selected)
				require.Equal(t, oldRevision, source.Commit)
				requireTestFile(t, filepath.Join(snapshotDirectory(root, selected), "stable.sk"), "old")
			}
			calls++
			writeTestFile(t, filepath.Join(paths.destination, "stable.sk"), "new")
			return nil
		}
		generators[language] = definition
	}
	results, err := Update(t.Context(), root, "", "", false, io.Discard, io.Discard)
	require.NoError(t, err)
	require.Len(t, results, len(Languages()))
	require.Equal(t, len(Languages()), calls)
	config, _, err := loadConfig(root)
	require.NoError(t, err)
	for _, language := range Languages() {
		source, _ := config.source(language)
		require.Equal(t, newRevision, source.Commit)
		requireTestFile(t, filepath.Join(snapshotDirectory(root, language), "stable.sk"), "new")
	}
}

func TestUpdateGenerationFailureLeavesEntireBatchUntouched(t *testing.T) {
	root, oldRevision, _ := updateFixture(t)
	_, before, err := loadConfig(root)
	require.NoError(t, err)
	definition := generators["java"]
	definition.generate = func(context.Context, generationPaths, commandRunner) error {
		return errors.New("upstream generation failed")
	}
	generators["java"] = definition
	results, err := Update(t.Context(), root, "", "", false, io.Discard, io.Discard)
	require.ErrorContains(t, err, "update aborted before publishing")
	require.Nil(t, results)
	config, after, err := loadConfig(root)
	require.NoError(t, err)
	require.Equal(t, before, after)
	for _, language := range Languages() {
		source, _ := config.source(language)
		require.Equal(t, oldRevision, source.Commit)
		requireTestFile(t, filepath.Join(snapshotDirectory(root, language), "stable.sk"), "old")
	}
}

func TestUpdateSkipsUnstableSourceWithinBatch(t *testing.T) {
	root, _, newRevision := updateFixture(t)
	config, _, err := loadConfig(root)
	require.NoError(t, err)
	require.NoError(t, config.setCommit("go", newRevision))
	content, err := encodeConfig(config)
	require.NoError(t, err)
	writeTestFile(t, filepath.Join(root, configFilename), string(content))
	target := snapshotDirectory(root, "go")
	writeTestFile(t, filepath.Join(target, "stable.sk"), "new")
	writeTestFile(t, filepath.Join(target, "unstable.sk"), "old")
	definition := generators["go"]
	generate := definition.generate
	definition.generate = func(ctx context.Context, paths generationPaths, runner commandRunner) error {
		if err := generate(ctx, paths, runner); err != nil {
			return err
		}
		writeTestFile(t, filepath.Join(paths.destination, "unstable.sk"), "new")
		return nil
	}
	definition.stability = func(path string) Stability {
		if path == "unstable.sk" {
			return Unstable
		}
		return Stable
	}
	generators["go"] = definition
	_, err = Update(t.Context(), root, "", "", true, io.Discard, io.Discard)
	require.NoError(t, err)
	config, _, err = loadConfig(root)
	require.NoError(t, err)
	for _, language := range Languages() {
		source, _ := config.source(language)
		require.Equal(t, newRevision, source.Commit)
		requireTestFile(t, filepath.Join(snapshotDirectory(root, language), "stable.sk"), "new")
	}
	requireTestFile(t, filepath.Join(target, "unstable.sk"), "old")
}

func TestUpdateUnstableOnlyPolicy(t *testing.T) {
	tests := []struct {
		name      string
		skip      bool
		advance   bool
		generated map[string]string
		keepOld   bool
	}{
		{name: "default refreshes unstable contents", generated: map[string]string{"stable.sk": "old", "unstable.sk": "new"}},
		{name: "skip retains unchanged source", skip: true, generated: map[string]string{"stable.sk": "old", "unstable.sk": "new"}, keepOld: true},
		{name: "new revision includes unstable contents", skip: true, advance: true, generated: map[string]string{"stable.sk": "old", "unstable.sk": "new"}},
		{name: "stable change includes unstable contents", skip: true, generated: map[string]string{"stable.sk": "new", "unstable.sk": "new"}},
		{name: "unstable addition is adopted", skip: true, generated: map[string]string{"stable.sk": "old", "unstable.sk": "new", "added.sk": "new"}},
		{name: "unstable deletion is adopted", skip: true, generated: map[string]string{"stable.sk": "old"}},
		{name: "revision only", skip: true, advance: true, generated: map[string]string{"stable.sk": "old", "unstable.sk": "old"}},
		{name: "no changes", skip: true, generated: map[string]string{"stable.sk": "old", "unstable.sk": "old"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, oldRevision, newRevision := updateFixture(t)
			target := snapshotDirectory(root, "go")
			writeTestFile(t, filepath.Join(target, "unstable.sk"), "old")
			definition := generators["go"]
			definition.generate = func(_ context.Context, paths generationPaths, _ commandRunner) error {
				for name, content := range test.generated {
					writeTestFile(t, filepath.Join(paths.destination, name), content)
				}
				return nil
			}
			definition.stability = func(path string) Stability {
				if path == "stable.sk" {
					return Stable
				}
				return Unstable
			}
			generators["go"] = definition
			revision := oldRevision
			if test.advance {
				revision = newRevision
			}
			results, err := Update(t.Context(), root, "go", revision, test.skip, io.Discard, io.Discard)
			require.NoError(t, err)
			require.Len(t, results, 1)
			config, _, err := loadConfig(root)
			require.NoError(t, err)
			require.Equal(t, revision, config.Snapshot.Go.Commit)
			require.Equal(t, oldRevision, config.Snapshot.CPP.Commit)
			require.Equal(t, oldRevision, config.Snapshot.Java.Commit)
			expected := test.generated
			if test.keepOld {
				expected = map[string]string{"stable.sk": "old", "unstable.sk": "old"}
			}
			entries, err := os.ReadDir(target)
			require.NoError(t, err)
			require.Len(t, entries, len(expected))
			for name, content := range expected {
				requireTestFile(t, filepath.Join(target, name), content)
			}
		})
	}
}

func TestCheckUsesPinnedRevisionWithoutWriting(t *testing.T) {
	root, oldRevision, _ := updateFixture(t)
	_, before, err := loadConfig(root)
	require.NoError(t, err)
	result, err := Check(t.Context(), root, "go", io.Discard, io.Discard)
	require.NoError(t, err)
	require.Equal(t, oldRevision, result.Revision)
	require.Empty(t, result.Changes)
	_, after, err := loadConfig(root)
	require.NoError(t, err)
	require.Equal(t, before, after)
	requireTestFile(t, filepath.Join(snapshotDirectory(root, "go"), "stable.sk"), "old")
}

func TestUpdatePreservesConcurrentConfigEdit(t *testing.T) {
	root, _, _ := updateFixture(t)
	_, before, err := loadConfig(root)
	require.NoError(t, err)
	edited := append(bytes.Clone(before), []byte("\n# Maintainer edit\n")...)
	definition := generators["go"]
	generate := definition.generate
	definition.generate = func(ctx context.Context, paths generationPaths, runner commandRunner) error {
		require.NoError(t, os.WriteFile(filepath.Join(root, configFilename), edited, 0o644))
		return generate(ctx, paths, runner)
	}
	generators["go"] = definition
	_, err = Update(t.Context(), root, "go", "", false, io.Discard, io.Discard)
	require.ErrorContains(t, err, "config.toml changed during generation")
	after, err := os.ReadFile(filepath.Join(root, configFilename))
	require.NoError(t, err)
	require.Equal(t, edited, after)
	requireTestFile(t, filepath.Join(snapshotDirectory(root, "go"), "stable.sk"), "old")
}

func updateFixture(t *testing.T) (string, string, string) {
	t.Helper()
	source := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		args = append([]string{"-C", source, "-c", "user.name=TCK test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)
		output, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, "%s", output)
		return strings.TrimSpace(string(output))
	}
	git("init", "--quiet")
	writeTestFile(t, filepath.Join(source, "stable.sk"), "old")
	git("add", "--force", "stable.sk")
	git("commit", "--quiet", "-m", "Initial snapshot")
	oldRevision := git("rev-parse", "HEAD")
	writeTestFile(t, filepath.Join(source, "stable.sk"), "new")
	git("commit", "--quiet", "-am", "Update snapshot")
	newRevision := git("rev-parse", "HEAD")

	original := generators
	t.Cleanup(func() { generators = original })
	generators = make(map[string]generator)
	for language := range original {
		generators[language] = generator{
			requirements: []string{"git"},
			generate: func(_ context.Context, paths generationPaths, _ commandRunner) error {
				if err := os.MkdirAll(paths.destination, 0o755); err != nil {
					return err
				}
				return copyFile(filepath.Join(paths.source, "stable.sk"), filepath.Join(paths.destination, "stable.sk"))
			},
			stability: func(string) Stability { return Stable },
		}
	}
	root := t.TempDir()
	pin := snapshotSource{Repository: source, Commit: oldRevision}
	content, err := encodeConfig(repositoryConfig{Snapshot: snapshotSources{CPP: pin, Go: pin, Java: pin}})
	require.NoError(t, err)
	writeTestFile(t, filepath.Join(root, configFilename), string(content))
	for _, language := range Languages() {
		writeTestFile(t, filepath.Join(snapshotDirectory(root, language), "stable.sk"), "old")
	}
	return root, oldRevision, newRevision
}
