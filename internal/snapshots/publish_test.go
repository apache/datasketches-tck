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
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublishStagingFailureDoesNotChangeTargets(t *testing.T) {
	root := t.TempDir()
	target, next := filepath.Join(root, "config.toml"), filepath.Join(root, "next")
	writeTestFile(t, target, "old")
	writeTestFile(t, next, "new")
	err := publish(root, []replacement{
		{target: target, source: next},
		{target: filepath.Join(root, "snapshots"), source: filepath.Join(root, "missing")},
	}, io.Discard)
	require.Error(t, err)
	requireTestFile(t, target, "old")
	require.NoDirExists(t, filepath.Join(root, "snapshots"))
}

func TestPublishRollsBackEarlierFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	nextFile, nextDir := filepath.Join(root, "next-config"), filepath.Join(root, "next-snapshots")
	writeTestFile(t, nextFile, "new config")
	writeTestFile(t, filepath.Join(nextDir, "new.sk"), "new")
	config, snapshots := filepath.Join(root, "config.toml"), filepath.Join(root, "snapshots")
	absent, locked := filepath.Join(root, "absent"), filepath.Join(root, "locked", "target")
	writeTestFile(t, config, "old config")
	writeTestFile(t, filepath.Join(snapshots, "old.sk"), "old")
	writeTestFile(t, locked, "old locked file")
	protectDirectory(t, filepath.Dir(locked))
	err := publish(root, []replacement{
		{target: config, source: nextFile},
		{target: snapshots, source: nextDir},
		{target: absent, source: nextDir},
		{target: locked, source: nextFile},
	}, io.Discard)
	require.ErrorContains(t, err, "preserve")
	requireTestFile(t, config, "old config")
	requireTestFile(t, filepath.Join(snapshots, "old.sk"), "old")
	require.NoFileExists(t, filepath.Join(snapshots, "new.sk"))
	require.NoDirExists(t, absent)
	requireTestFile(t, locked, "old locked file")
}

func TestPublishCleanupFailureKeepsNewConfigAndSnapshots(t *testing.T) {
	root := t.TempDir()
	target, config := filepath.Join(root, "snapshots"), filepath.Join(root, "config.toml")
	nextDir, nextFile := filepath.Join(root, "next-snapshots"), filepath.Join(root, "next-config")
	writeTestFile(t, filepath.Join(target, "protected", "old.sk"), "old")
	writeTestFile(t, config, "old revision")
	writeTestFile(t, filepath.Join(nextDir, "new.sk"), "new")
	writeTestFile(t, nextFile, "new revision")
	protectDirectory(t, filepath.Join(target, "protected"))
	// The protected directory moves into the transaction backup.
	t.Cleanup(func() {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(path, 0o755)
			}
			return nil
		})
	})
	var warnings bytes.Buffer
	require.NoError(t, publish(root, []replacement{
		{target: target, source: nextDir},
		{target: config, source: nextFile},
	}, &warnings))
	require.Contains(t, warnings.String(), "Warning: could not remove update backups")
	requireTestFile(t, config, "new revision")
	requireTestFile(t, filepath.Join(target, "new.sk"), "new")
	require.NoDirExists(t, filepath.Join(target, "protected"))
}

func protectDirectory(t *testing.T, directory string) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	require.NoError(t, os.Chmod(directory, 0o555))
	t.Cleanup(func() { _ = os.Chmod(directory, 0o755) })
}
