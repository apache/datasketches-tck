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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testCPPRevision  = "1111111111111111111111111111111111111111"
	testGoRevision   = "2222222222222222222222222222222222222222"
	testJavaRevision = "3333333333333333333333333333333333333333"
)

func TestRevisionConfigRoundTrip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	config := testRevisionConfig()
	content, err := encodeRevisionConfig(config)
	require.NoError(t, err)
	require.Contains(t, string(content), "Managed by: mise run tck")
	require.NoError(t, os.WriteFile(filepath.Join(root, revisionsFilename), content, 0o644))

	loaded, original, err := loadRevisionConfig(root)
	require.NoError(t, err)
	require.Equal(t, config, loaded)
	require.Equal(t, content, original)
}

func TestRevisionConfigRejectsUnknownLanguage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content, err := encodeRevisionConfig(testRevisionConfig())
	require.NoError(t, err)
	content = append(content, []byte("python = \"4444444444444444444444444444444444444444\"\n")...)
	require.NoError(t, os.WriteFile(filepath.Join(root, revisionsFilename), content, 0o644))

	_, _, err = loadRevisionConfig(root)
	require.Error(t, err)
}

func TestRevisionConfigRejectsSymbolicRevision(t *testing.T) {
	t.Parallel()

	config := testRevisionConfig()
	config.Revisions.Go = "main"
	_, err := encodeRevisionConfig(config)
	require.ErrorContains(t, err, "40-character lowercase commit ID")
}

func TestReplaceRevisionFilePreservesModeAndContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filename := filepath.Join(root, revisionsFilename)
	require.NoError(t, os.WriteFile(filename, []byte("before"), 0o644))

	require.NoError(t, replaceRevisionFile(root, []byte("after")))
	content, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, "after", string(content))
	info, err := os.Stat(filename)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), ".snapshot-revisions-"))
	}
}

func testRevisionConfig() revisionConfig {
	return revisionConfig{Revisions: revisionSet{
		CPP:  testCPPRevision,
		Go:   testGoRevision,
		Java: testJavaRevision,
	}}
}
