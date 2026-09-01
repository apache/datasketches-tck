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
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/apache/datasketches-tck/internal/snapshots"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

func printResult(output io.Writer, root string, mode snapshots.Mode, result snapshots.Result) error {
	target := displayPath(root, result.Target)
	if len(result.Changes) == 0 {
		if result.RevisionChanged() {
			if err := printRevisionChange(output, result); err != nil {
				return err
			}
			_, err := fmt.Fprintf(output, "✓ Updated %s.\n", target)
			return err
		}
		_, err := fmt.Fprintf(output, "✓ %s is up to date.\n", target)
		return err
	}

	counts := make(map[snapshots.ChangeStatus]int)
	unstableModified := 0
	changeTable := table.NewWriter()
	changeTable.SetStyle(table.StyleRounded)
	changeTable.SetColumnConfigs([]table.ColumnConfig{{
		Name: "Snapshot", WidthMax: 60, WidthMaxEnforcer: text.WrapHard,
	}})
	changeTable.AppendHeader(table.Row{"Change", "Stability", "Check", "Snapshot", "Content (size · sha256)"})
	for _, change := range result.Changes {
		counts[change.Status]++
		if change.Status == snapshots.ChangeModified && change.Stability == snapshots.Unstable {
			unstableModified++
		}
		changeTable.AppendRow(table.Row{
			change.Status,
			change.Stability,
			checkDisposition(change),
			change.Path,
			changeDetails(change),
		})
	}

	if _, err := fmt.Fprintln(output, changeTable.Render()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"Summary: %d added · %d modified (%d probabilistic) · %d deleted\n",
		counts[snapshots.ChangeAdded],
		counts[snapshots.ChangeModified],
		unstableModified,
		counts[snapshots.ChangeDeleted],
	); err != nil {
		return err
	}
	if result.RevisionChanged() {
		if err := printRevisionChange(output, result); err != nil {
			return err
		}
	}

	switch {
	case mode == snapshots.ModeUpdate:
		_, err := fmt.Fprintf(output, "✓ Updated %s.\n", target)
		return err
	case mode == snapshots.ModeSync:
		_, err := fmt.Fprintf(output, "✓ Synced %s.\n", target)
		return err
	case result.HasBlockingChanges():
		count := result.BlockingChangeCount()
		_, err := fmt.Fprintf(output, "✗ %s has %d blocking %s.\n", target, count, plural(count, "change", "changes"))
		return err
	default:
		_, err := fmt.Fprintf(
			output,
			"! Snapshot set and stable contents are unchanged; %d probabilistic content %s allowed.\n",
			unstableModified,
			plural(unstableModified, "modification is", "modifications are"),
		)
		return err
	}
}

func printRevisionChange(output io.Writer, result snapshots.Result) error {
	_, err := fmt.Fprintf(
		output,
		"Source revision: %s -> %s\n",
		result.PreviousRevision,
		result.Revision,
	)
	return err
}

func changeDetails(change snapshots.Change) string {
	switch change.Status {
	case snapshots.ChangeAdded:
		return "new " + formatMetadata(change.After)
	case snapshots.ChangeDeleted:
		return "old " + formatMetadata(change.Before)
	case snapshots.ChangeModified:
		return "old " + formatMetadata(change.Before) + "\nnew " + formatMetadata(change.After)
	default:
		return "unknown"
	}
}

func formatMetadata(metadata *snapshots.FileMetadata) string {
	if metadata == nil {
		return "—"
	}
	return fmt.Sprintf(
		"%d B · %s",
		metadata.Size,
		hex.EncodeToString(metadata.SHA256[:6]),
	)
}

func checkDisposition(change snapshots.Change) string {
	if change.IsBlocking() {
		return "block"
	}
	return "allow"
}

func plural(count int, singular, multiple string) string {
	if count == 1 {
		return singular
	}
	return multiple
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
