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
	"os"
	"path/filepath"
	"testing"
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
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3: %#v", len(changes), changes)
	}

	if got := changes[0]; got.Status != ChangeAdded || got.Path != "added.sk" || got.After == nil || got.After.Size != 5 {
		t.Fatalf("unexpected added change: %#v", got)
	}
	if got := changes[1]; got.Status != ChangeDeleted || got.Path != "deleted.sk" || got.Before == nil || got.Before.Size != 7 {
		t.Fatalf("unexpected deleted change: %#v", got)
	}
	if got := changes[2]; got.Status != ChangeModified || got.Path != "modified.sk" || got.Stability != Unstable {
		t.Fatalf("unexpected modified change: %#v", got)
	}
	if changes[2].Before == nil || changes[2].After == nil || changes[2].Before.SHA256 == changes[2].After.SHA256 {
		t.Fatalf("modified change lacks distinct hashes: %#v", changes[2])
	}
}

func TestResultOnlyAllowsUnstableModifications(t *testing.T) {
	t.Parallel()

	result := Result{Changes: []Change{{Status: ChangeModified, Stability: Unstable}}}
	if result.HasBlockingChanges() {
		t.Fatal("unstable modification should not block check mode")
	}

	result.Changes = append(result.Changes, Change{Status: ChangeAdded, Stability: Unstable})
	result.Changes = append(result.Changes, Change{Status: ChangeModified, Stability: Stable})
	result.Changes = append(result.Changes, Change{Status: ChangeDeleted, Stability: Stable})
	if got := result.BlockingChangeCount(); got != 3 {
		t.Fatalf("got %d blocking changes, want 3", got)
	}
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

	if err := replaceDirectory(target, generated); err != nil {
		t.Fatal(err)
	}
	requireTestFile(t, filepath.Join(target, "modified.sk"), "new")
	requireTestFile(t, filepath.Join(target, "nested", "added.sk"), "added")
	if _, err := os.Stat(filepath.Join(target, "deleted.sk")); !os.IsNotExist(err) {
		t.Fatalf("deleted file still exists or stat failed unexpectedly: %v", err)
	}
}

func writeTestFile(t *testing.T, filename, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireTestFile(t *testing.T, filename, expected string) {
	t.Helper()
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != expected {
		t.Fatalf("%s contains %q, want %q", filename, content, expected)
	}
}
