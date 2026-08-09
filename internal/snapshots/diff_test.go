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
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompareDirectoriesReportsDetailedChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	current := filepath.Join(root, "current")
	generated := filepath.Join(root, "generated")
	writeTestFile(t, filepath.Join(current, "deleted.sk"), "deleted")
	writeTestFile(t, filepath.Join(current, "modified.sk"), "old")
	writeTestFile(t, filepath.Join(current, "same.sk"), "same")
	writeTestFile(t, filepath.Join(generated, "added.sk"), "added")
	writeTestFile(t, filepath.Join(generated, "modified.sk"), "new")
	writeTestFile(t, filepath.Join(generated, "same.sk"), "same")

	changes, err := compareDirectories(current, generated, func(path string) Stability {
		if path == "modified.sk" {
			return Unstable
		}
		return Stable
	})

	added := testFileMetadata("added")
	deleted := testFileMetadata("deleted")
	before := testFileMetadata("old")
	after := testFileMetadata("new")
	want := []Change{
		{Status: ChangeAdded, Stability: Stable, Path: "added.sk", After: &added},
		{Status: ChangeDeleted, Stability: Stable, Path: "deleted.sk", Before: &deleted},
		{Status: ChangeModified, Stability: Unstable, Path: "modified.sk", Before: &before, After: &after},
	}
	require.NoError(t, err)
	require.Equal(t, want, changes)
}

func TestResultOnlyAllowsUnstableModifications(t *testing.T) {
	t.Parallel()

	result := Result{Changes: []Change{{Status: ChangeModified, Stability: Unstable}}}
	require.False(t, result.HasBlockingChanges())

	result.Changes = append(result.Changes, Change{Status: ChangeAdded, Stability: Unstable})
	result.Changes = append(result.Changes, Change{Status: ChangeModified, Stability: Stable})
	result.Changes = append(result.Changes, Change{Status: ChangeDeleted, Stability: Stable})
	require.Equal(t, 3, result.BlockingChangeCount())
}

func TestReplaceDirectoryReplacesTheExactFileSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "snapshots")
	generated := filepath.Join(root, "generated")
	writeTestFile(t, filepath.Join(target, "deleted.sk"), "deleted")
	writeTestFile(t, filepath.Join(target, "modified.sk"), "old")
	writeTestFile(t, filepath.Join(generated, "modified.sk"), "new")
	writeTestFile(t, filepath.Join(generated, "nested", "added.sk"), "added")

	require.NoError(t, replaceDirectory(target, generated))
	requireTestFile(t, filepath.Join(target, "modified.sk"), "new")
	requireTestFile(t, filepath.Join(target, "nested", "added.sk"), "added")
	require.NoFileExists(t, filepath.Join(target, "deleted.sk"))
}

func writeTestFile(t *testing.T, filename, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0o755))
	require.NoError(t, os.WriteFile(filename, []byte(content), 0o644))
}

func requireTestFile(t *testing.T, filename, expected string) {
	t.Helper()
	content, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, expected, string(content))
}

func testFileMetadata(content string) FileMetadata {
	return FileMetadata{Size: int64(len(content)), SHA256: sha256.Sum256([]byte(content))}
}
