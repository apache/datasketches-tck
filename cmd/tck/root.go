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
	"io"

	"github.com/spf13/cobra"
)

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	command := newRootCommand()
	command.SetArgs(args)
	command.SetOut(stdout)
	command.SetErr(stderr)

	err := command.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	if errors.Is(err, errSnapshotsOutOfDate) {
		return 1
	}

	command.PrintErrf("Error: %v\n", err)
	return 1
}

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "tck",
		Short:         "Generate and validate DataSketches serialization snapshots",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	command.CompletionOptions.DisableDefaultCmd = true
	command.AddCommand(newSnapshotsCommand())
	return command
}
