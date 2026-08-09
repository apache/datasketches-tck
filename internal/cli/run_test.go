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

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apache/datasketches-tck/internal/snapshots"
)

func TestPrintResultIncludesDetailedFileChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	before := testMetadata(12, 0xaa)
	after := testMetadata(16, 0xbb)
	result := snapshots.Result{
		Target: filepath.Join(root, "serialization", "go", "snapshots"),
		Changes: []snapshots.Change{
			{
				Status: snapshots.ChangeAdded, Stability: snapshots.Stable,
				Path: "added.sk", After: &after,
			},
			{
				Status: snapshots.ChangeModified, Stability: snapshots.Unstable,
				Path: "modified.sk", Before: &before, After: &after,
			},
			{
				Status: snapshots.ChangeDeleted, Stability: snapshots.Stable,
				Path: "deleted.sk", Before: &before,
			},
		},
	}

	var output bytes.Buffer
	printResult(&output, root, snapshots.ModeCheck, result)
	wantLines := []string{
		"A stable   added.sk (16 B, sha256:bbbbbbbbbbbb)",
		"M unstable modified.sk (12 B, sha256:aaaaaaaaaaaa -> 16 B, sha256:bbbbbbbbbbbb)",
		"D stable   deleted.sk (12 B, sha256:aaaaaaaaaaaa)",
		"Summary: 1 added, 1 modified (1 unstable), 1 deleted.",
		"serialization/go/snapshots has 2 blocking change(s).",
	}
	for _, line := range wantLines {
		if !strings.Contains(output.String(), line+"\n") {
			t.Fatalf("output does not contain %q:\n%s", line, output.String())
		}
	}
}

func TestPrintResultAllowsOnlyUnstableModifications(t *testing.T) {
	t.Parallel()

	before := testMetadata(12, 0xaa)
	after := testMetadata(12, 0xbb)
	result := snapshots.Result{
		Target: "snapshots",
		Changes: []snapshots.Change{{
			Status: snapshots.ChangeModified, Stability: snapshots.Unstable,
			Path: "probabilistic.sk", Before: &before, After: &after,
		}},
	}

	var output bytes.Buffer
	printResult(&output, ".", snapshots.ModeCheck, result)
	if !strings.Contains(output.String(), "Stable snapshots are current; 1 unstable modification(s) are allowed.\n") {
		t.Fatalf("unexpected output:\n%s", output.String())
	}
}

func testMetadata(size int64, fill byte) snapshots.FileMetadata {
	metadata := snapshots.FileMetadata{Size: size}
	for index := range metadata.SHA256 {
		metadata.SHA256[index] = fill
	}
	return metadata
}
