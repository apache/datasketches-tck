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
		newSnapshotReconcileCommand(snapshots.ModeCheck),
		newSnapshotReconcileCommand(snapshots.ModeUpdate),
	)
	return command
}

func newSnapshotReconcileCommand(mode snapshots.Mode) *cobra.Command {
	verb := string(mode)
	validLanguages := append(snapshots.Languages(), "all")
	var revision string
	command := &cobra.Command{
		Use:       verb + " <cpp|go|java|all>",
		Short:     reconcileDescription(mode),
		Long:      reconcileDetails(mode),
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: validLanguages,
		RunE: func(command *cobra.Command, args []string) error {
			return reconcileSnapshots(command, mode, args[0], revision)
		},
	}
	if mode == snapshots.ModeUpdate {
		command.Flags().StringVar(
			&revision,
			"revision",
			"",
			"upstream commit, branch, or tag to adopt",
		)
	}
	return command
}

func reconcileSnapshots(
	command *cobra.Command,
	mode snapshots.Mode,
	requestedLanguage, requestedRevision string,
) error {
	if requestedLanguage == "all" && requestedRevision != "" {
		return fmt.Errorf("--revision cannot be used with all; update each language separately")
	}
	languages := []string{requestedLanguage}
	if requestedLanguage == "all" {
		languages = snapshots.Languages()
	}

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
	case snapshots.ModeUpdate:
		return "Replace committed snapshots with newly generated snapshots"
	default:
		panic(fmt.Sprintf("unsupported snapshot mode %q", mode))
	}
}

func reconcileDetails(mode snapshots.Mode) string {
	switch mode {
	case snapshots.ModeCheck:
		return "Generate snapshots and verify that the file set and stable contents match. Content changes to existing probabilistic snapshots are reported but do not fail the check."
	case snapshots.ModeUpdate:
		return "Generate snapshots and atomically replace the selected committed snapshot directory with the complete generated file set. Pass --revision to adopt an upstream commit, branch, or tag and record its resolved commit ID."
	default:
		panic(fmt.Sprintf("unsupported snapshot mode %q", mode))
	}
}

func modeHeading(mode snapshots.Mode) string {
	switch mode {
	case snapshots.ModeCheck:
		return "Check"
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
