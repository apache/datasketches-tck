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

package main

import (
	"bytes"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
)

func TestRunPrintsSnapshotsHelp(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	var stderr bytes.Buffer
	require.Equal(t, 0, run(t.Context(), []string{"snapshots"}, &output, &stderr))
	require.Empty(t, stderr.String())
	snaps.WithConfig(snaps.Raw()).MatchSnapshot(t, output.String())
}

func TestRunRejectsUnsupportedLanguage(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(t.Context(), []string{"snapshots", "check", "python"}, &output, &stderr)

	require.Equal(t, 1, exitCode)
	require.Empty(t, output.String())
	require.Contains(t, stderr.String(), `invalid argument "python" for "tck snapshots check"`)
}

func TestRunRejectsOneRevisionForAllLanguages(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(
		t.Context(),
		[]string{"snapshots", "update", "all", "--revision", "main"},
		&output,
		&stderr,
	)

	require.Equal(t, 1, exitCode)
	require.Empty(t, output.String())
	require.Contains(t, stderr.String(), "--revision cannot be used with all")
}
