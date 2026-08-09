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
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/apache/datasketches-tck/internal/snapshots"
)

var errSnapshotsOutOfDate = errors.New("stable snapshots are out of date")

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if err := run(ctx, args, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || (len(args) == 1 && isHelp(args[0])) {
		printUsage(stdout)
		return nil
	}
	if len(args) != 3 || args[0] != "snapshots" {
		printUsage(stderr)
		return errors.New("expected snapshots <check|update> <cpp|go|java|all>")
	}

	mode := snapshots.Mode(args[1])
	if mode != snapshots.ModeCheck && mode != snapshots.ModeUpdate {
		return fmt.Errorf("unsupported snapshot mode %q", args[1])
	}

	languages := []string{args[2]}
	if args[2] == "all" {
		languages = snapshots.Languages()
	} else if !snapshots.HasLanguage(args[2]) {
		return fmt.Errorf("unsupported snapshot language %q", args[2])
	}

	root, err := repositoryRoot(ctx)
	if err != nil {
		return err
	}

	outOfDate := false
	for _, language := range languages {
		fmt.Fprintf(stdout, "--- %s %s snapshots ---\n", title(args[1]), language)
		result, err := snapshots.Reconcile(ctx, root, language, mode, stdout, stderr)
		if err != nil {
			return err
		}
		printResult(stdout, root, mode, result)
		outOfDate = outOfDate || (mode == snapshots.ModeCheck && result.HasBlockingChanges())
	}
	if outOfDate {
		return errSnapshotsOutOfDate
	}
	return nil
}

func printResult(output io.Writer, root string, mode snapshots.Mode, result snapshots.Result) {
	target := displayPath(root, result.Target)
	if len(result.Changes) == 0 {
		fmt.Fprintf(output, "%s is up to date.\n", target)
		return
	}

	counts := make(map[snapshots.ChangeStatus]int)
	unstableModified := 0
	for _, change := range result.Changes {
		counts[change.Status]++
		if change.Status == snapshots.ChangeModified && change.Stability == snapshots.Unstable {
			unstableModified++
		}
		fmt.Fprintf(
			output,
			"%s %-8s %s (%s)\n",
			change.Status,
			change.Stability,
			change.Path,
			changeDetails(change),
		)
	}

	fmt.Fprintf(
		output,
		"Summary: %d added, %d modified (%d unstable), %d deleted.\n",
		counts[snapshots.ChangeAdded],
		counts[snapshots.ChangeModified],
		unstableModified,
		counts[snapshots.ChangeDeleted],
	)

	switch {
	case mode == snapshots.ModeUpdate:
		fmt.Fprintf(output, "Updated %s.\n", target)
	case result.HasBlockingChanges():
		fmt.Fprintf(output, "%s has %d blocking change(s).\n", target, result.BlockingChangeCount())
	default:
		fmt.Fprintf(output, "Stable snapshots are current; %d unstable modification(s) are allowed.\n", unstableModified)
	}
}

func changeDetails(change snapshots.Change) string {
	switch change.Status {
	case snapshots.ChangeAdded:
		return formatMetadata(*change.After)
	case snapshots.ChangeDeleted:
		return formatMetadata(*change.Before)
	case snapshots.ChangeModified:
		return formatMetadata(*change.Before) + " -> " + formatMetadata(*change.After)
	default:
		panic(fmt.Sprintf("unsupported change status %q", change.Status))
	}
}

func formatMetadata(metadata snapshots.FileMetadata) string {
	return fmt.Sprintf(
		"%d B, sha256:%s",
		metadata.Size,
		hex.EncodeToString(metadata.SHA256[:6]),
	)
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

func displayPath(root, filename string) string {
	relative, err := filepath.Rel(root, filename)
	if err != nil || relative == ".." || filepath.IsAbs(relative) {
		return filename
	}
	if strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filename
	}
	return filepath.ToSlash(relative)
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage: tck snapshots <check|update> <cpp|go|java|all>")
}

func isHelp(argument string) bool {
	return argument == "help" || argument == "-h" || argument == "--help"
}

func title(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
