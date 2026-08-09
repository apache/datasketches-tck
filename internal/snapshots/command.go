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
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func (runner commandRunner) run(ctx context.Context, directory, name string, arguments ...string) error {
	commandLine := strings.Join(append([]string{name}, arguments...), " ")
	fmt.Fprintf(runner.stdout, "$ %s\n", commandLine)

	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	command.Stdout = runner.stdout
	command.Stderr = runner.stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", commandLine, err)
	}
	return nil
}

func cloneAtCommit(
	ctx context.Context,
	runner commandRunner,
	repository, commit, destination string,
) error {
	if err := runner.run(ctx, "", "git", "init", "--quiet", destination); err != nil {
		return err
	}
	if err := runner.run(ctx, "", "git", "-C", destination, "remote", "add", "origin", repository); err != nil {
		return err
	}
	if err := runner.run(ctx, "", "git", "-C", destination, "fetch", "--depth", "1", "--no-tags", "origin", commit); err != nil {
		return err
	}
	if err := runner.run(ctx, "", "git", "-C", destination, "checkout", "--quiet", "--detach", "FETCH_HEAD"); err != nil {
		return err
	}
	return nil
}
