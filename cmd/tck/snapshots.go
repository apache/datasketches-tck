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
		newSnapshotSyncCommand(),
		newSnapshotUpdateCommand(),
	)
	return command
}

func newSnapshotCheckCommand() *cobra.Command {
	validLanguages := append(snapshots.Languages(), "all")
	return &cobra.Command{
		Use:       "check <cpp|go|java|all>",
		Short:     reconcileDescription(snapshots.ModeCheck),
		Long:      reconcileDetails(snapshots.ModeCheck),
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: validLanguages,
		RunE: func(command *cobra.Command, args []string) error {
			languages := []string{args[0]}
			if args[0] == "all" {
				languages = snapshots.Languages()
			}
			return reconcileSnapshots(command, snapshots.ModeCheck, languages, "")
		},
	}
}

func newSnapshotSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: reconcileDescription(snapshots.ModeSync),
		Long:  reconcileDetails(snapshots.ModeSync),
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return reconcileSnapshots(command, snapshots.ModeSync, snapshots.Languages(), "")
		},
	}
}

func newSnapshotUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update <cpp|go|java> <revision>",
		Short: reconcileDescription(snapshots.ModeUpdate),
		Long:  reconcileDetails(snapshots.ModeUpdate),
		Args: func(command *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(2)(command, args); err != nil {
				return err
			}
			for _, language := range snapshots.Languages() {
				if args[0] == language {
					return nil
				}
			}
			return fmt.Errorf("unsupported snapshot language %q", args[0])
		},
		ValidArgs: snapshots.Languages(),
		RunE: func(command *cobra.Command, args []string) error {
			return reconcileSnapshots(command, snapshots.ModeUpdate, []string{args[0]}, args[1])
		},
	}
}

func reconcileSnapshots(
	command *cobra.Command,
	mode snapshots.Mode,
	languages []string,
	requestedRevision string,
) error {
	root, err := repositoryRoot(command.Context())
	if err != nil {
		return err
	}

	outOfDate := false
	for index, language := range languages {
		if index > 0 {
			if _, err := fmt.Fprintln(command.OutOrStdout()); err != nil {
				return fmt.Errorf("separate snapshot results: %w", err)
			}
		}
		if _, err := fmt.Fprintf(
			command.OutOrStdout(),
			"%s %s snapshots\n",
			modeHeading(mode),
			languageHeading(language),
		); err != nil {
			return fmt.Errorf("print snapshot heading: %w", err)
		}

		result, err := snapshots.Reconcile(
			command.Context(),
			root,
			language,
			mode,
			requestedRevision,
			command.OutOrStdout(),
			command.ErrOrStderr(),
		)
		if err != nil {
			return err
		}
		if err := printResult(command.OutOrStdout(), root, mode, result); err != nil {
			return fmt.Errorf("print snapshot result: %w", err)
		}
		outOfDate = outOfDate || (mode == snapshots.ModeCheck && result.HasBlockingChanges())
	}
	if outOfDate {
		return errSnapshotsOutOfDate
	}
	return nil
}

func reconcileDescription(mode snapshots.Mode) string {
	switch mode {
	case snapshots.ModeCheck:
		return "Check the snapshot set and stable snapshot contents"
	case snapshots.ModeSync:
		return "Synchronize snapshots from config.toml"
	case snapshots.ModeUpdate:
		return "Adopt one upstream revision and update its snapshots"
	default:
		panic(fmt.Sprintf("unsupported snapshot mode %q", mode))
	}
}

func reconcileDetails(mode snapshots.Mode) string {
	switch mode {
	case snapshots.ModeCheck:
		return "Generate snapshots and verify that the file set and stable contents match. Content changes to existing probabilistic snapshots are reported but do not fail the check."
	case snapshots.ModeSync:
		return "Regenerate all snapshot directories from the repositories and commits in config.toml without changing the config."
	case snapshots.ModeUpdate:
		return "Resolve an upstream commit, branch, or tag, record its exact commit ID in config.toml, and regenerate that source's snapshots."
	default:
		panic(fmt.Sprintf("unsupported snapshot mode %q", mode))
	}
}

func modeHeading(mode snapshots.Mode) string {
	switch mode {
	case snapshots.ModeCheck:
		return "Check"
	case snapshots.ModeSync:
		return "Sync"
	case snapshots.ModeUpdate:
		return "Update"
	default:
		panic(fmt.Sprintf("unsupported snapshot mode %q", mode))
	}
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
