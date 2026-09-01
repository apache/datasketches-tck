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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReconcileUpdateAdoptsResolvedRevisionAndSnapshots(t *testing.T) {
	source, oldRevision, newRevision := createTestSnapshotRepository(t, "old", "new")
	root := createTestTCKRepository(t, oldRevision, "old")
	replaceTestGenerator(t, source, nil)

	result, err := Reconcile(
		t.Context(), root, "go", ModeUpdate, "main", io.Discard, io.Discard,
	)
	require.NoError(t, err)
	require.Equal(t, oldRevision, result.PreviousRevision)
	require.Equal(t, newRevision, result.Revision)
	require.True(t, result.RevisionChanged())
	require.Len(t, result.Changes, 1)
	requireTestFile(t, filepath.Join(root, "serialization", "go", "snapshots", "snapshot.sk"), "new")

	config, _, err := loadRevisionConfig(root)
	require.NoError(t, err)
	actualRevision, found := config.revision("go")
	require.True(t, found)
	require.Equal(t, newRevision, actualRevision)
}

func TestReconcileUpdateAdvancesRevisionWithoutSnapshotChanges(t *testing.T) {
	source, oldRevision, newRevision := createTestSnapshotRepository(t, "same", "same")
	root := createTestTCKRepository(t, oldRevision, "same")
	replaceTestGenerator(t, source, nil)

	result, err := Reconcile(
		t.Context(), root, "go", ModeUpdate, "main", io.Discard, io.Discard,
	)
	require.NoError(t, err)
	require.Equal(t, oldRevision, result.PreviousRevision)
	require.Equal(t, newRevision, result.Revision)
	require.Empty(t, result.Changes)

	config, _, err := loadRevisionConfig(root)
	require.NoError(t, err)
	actualRevision, found := config.revision("go")
	require.True(t, found)
	require.Equal(t, newRevision, actualRevision)
}

func TestReconcileUpdateLeavesRepositoryUntouchedWhenGenerationFails(t *testing.T) {
	source, oldRevision, _ := createTestSnapshotRepository(t, "old", "new")
	root := createTestTCKRepository(t, oldRevision, "old")
	expectedRevisions, err := os.ReadFile(filepath.Join(root, revisionsFilename))
	require.NoError(t, err)
	replaceTestGenerator(t, source, errors.New("generator failed"))

	_, err = Reconcile(t.Context(), root, "go", ModeUpdate, "main", io.Discard, io.Discard)
	require.ErrorContains(t, err, "generator failed")
	requireTestFile(t, filepath.Join(root, "serialization", "go", "snapshots", "snapshot.sk"), "old")
	actualRevisions, readErr := os.ReadFile(filepath.Join(root, revisionsFilename))
	require.NoError(t, readErr)
	require.Equal(t, expectedRevisions, actualRevisions)
}

func createTestSnapshotRepository(t *testing.T, first, second string) (string, string, string) {
	t.Helper()

	repository := t.TempDir()
	runTestGit(t, repository, "init", "--quiet", "--initial-branch", "main")
	runTestGit(t, repository, "config", "user.name", "TCK Test")
	runTestGit(t, repository, "config", "user.email", "tck@example.invalid")
	filename := filepath.Join(repository, "snapshot.sk")
	require.NoError(t, os.WriteFile(filename, []byte(first), 0o644))
	runTestGit(t, repository, "add", "snapshot.sk")
	runTestGit(t, repository, "commit", "--quiet", "-m", "first")
	oldRevision := strings.TrimSpace(runTestGit(t, repository, "rev-parse", "HEAD"))

	require.NoError(t, os.WriteFile(filename, []byte(second), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repository, "metadata.txt"), []byte("second"), 0o644))
	runTestGit(t, repository, "add", ".")
	runTestGit(t, repository, "commit", "--quiet", "-m", "second")
	newRevision := strings.TrimSpace(runTestGit(t, repository, "rev-parse", "HEAD"))
	return repository, oldRevision, newRevision
}

func createTestTCKRepository(t *testing.T, goRevision, snapshot string) string {
	t.Helper()

	root := t.TempDir()
	config := testRevisionConfig()
	config.Revisions.Go = goRevision
	content, err := encodeRevisionConfig(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, revisionsFilename), content, 0o644))
	writeTestFile(t, filepath.Join(root, "serialization", "go", "snapshots", "snapshot.sk"), snapshot)
	return root
}

func replaceTestGenerator(t *testing.T, repository string, generateErr error) {
	t.Helper()

	original := generators["go"]
	t.Cleanup(func() { generators["go"] = original })
	generators["go"] = generator{
		repository: repository,
		generate: func(_ context.Context, paths generationPaths, _ commandRunner) error {
			if generateErr != nil {
				return generateErr
			}
			return collectSnapshots(paths.source, paths.destination, func(name string) bool {
				return name == "snapshot.sk"
			})
		},
		stability: func(string) Stability { return Stable },
	}
}

func runTestGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}
