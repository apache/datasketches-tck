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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/apache/datasketches-tck/internal/snapshots"
	"github.com/spf13/cobra"
)

var errSnapshotsOutOfDate = errors.New("snapshots contain blocking changes")

func newSnapshotsCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "snapshots",
		Short: "Generate, compare, and update serialization snapshots",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	command.AddCommand(
		newSnapshotCheckCommand(),
		newSnapshotUpdateCommand(),
	)
	return command
}

func newSnapshotCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "check <cpp|go|java|all>",
		Short:     "Check the snapshot set and stable snapshot contents",
		Long:      "Regenerate snapshots at the configured commits without writing files. Additions, deletions, and stable content changes fail the check; unstable content changes are allowed.",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: append(snapshots.Languages(), "all"),
		RunE: func(command *cobra.Command, args []string) error {
			root, err := repositoryRoot(command.Context())
			if err != nil {
				return err
			}
			languages := []string{args[0]}
			if args[0] == "all" {
				languages = snapshots.Languages()
			}
			outOfDate := false
			for _, language := range languages {
				result, err := snapshots.Check(command.Context(), root, language, command.OutOrStdout(), command.ErrOrStderr())
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintf(command.OutOrStdout(), "\nCheck %s snapshots\n", languageHeading(language)); err != nil {
					return err
				}
				if err := printCheckResult(command.OutOrStdout(), root, result); err != nil {
					return err
				}
				outOfDate = outOfDate || result.HasBlockingChanges()
			}
			if outOfDate {
				return errSnapshotsOutOfDate
			}
			return nil
		},
	}
}

func newSnapshotUpdateCommand() *cobra.Command {
	var skipUnstableOnly bool
	command := &cobra.Command{
		Use:   "update [cpp|go|java] [revision]",
		Short: "Update snapshots and source revisions together",
		Long:  "Update all sources to their remote HEADs, or select one language and optionally a branch, tag, or commit. All selected sources are generated before any snapshots or pins are published.",
		Args: func(command *cobra.Command, args []string) error {
			if err := cobra.RangeArgs(0, 2)(command, args); err != nil {
				return err
			}
			if len(args) > 0 && !slices.Contains(snapshots.Languages(), args[0]) {
				return fmt.Errorf("unsupported snapshot language %q", args[0])
			}
			return nil
		},
		ValidArgs: snapshots.Languages(),
		RunE: func(command *cobra.Command, args []string) error {
			root, err := repositoryRoot(command.Context())
			if err != nil {
				return err
			}
			language, revision := "", ""
			if len(args) > 0 {
				language = args[0]
			}
			if len(args) > 1 {
				revision = args[1]
			}
			results, err := snapshots.Update(command.Context(), root, language, revision, skipUnstableOnly, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			for _, result := range results {
				if _, err := fmt.Fprintf(command.OutOrStdout(), "\nUpdate %s\n", displayPath(root, result.Target)); err != nil {
					return err
				}
				if err := printUpdateResult(command.OutOrStdout(), root, result, skipUnstableOnly); err != nil {
					return err
				}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&skipUnstableOnly, "skip-unstable-only", false,
		"Keep existing snapshots and source revision unless files are added, deleted, or stable contents change")
	return command
}

func languageHeading(language string) string {
	switch language {
	case "cpp":
		return "C++"
	case "go":
		return "Go"
	case "java":
		return "Java"
	default:
		return language
	}
}

func repositoryRoot(ctx context.Context) (string, error) {
	if configured := os.Getenv("TCK_REPOSITORY_ROOT"); configured != "" {
		root, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf("resolve TCK_REPOSITORY_ROOT: %w", err)
		}
		return root, nil
	}

	command := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository root with git: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
