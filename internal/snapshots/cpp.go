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
	"os"
	"path/filepath"
	"strings"
)

func generateCPP(
	ctx context.Context,
	paths generationPaths,
	runner commandRunner,
) error {
	build := filepath.Join(paths.workspace, "build")
	if err := os.Mkdir(build, 0o755); err != nil {
		return err
	}
	if err := runner.run(
		ctx,
		"",
		"cmake",
		"-S", paths.source,
		"-B", build,
		"-DGENERATE=true",
		"-DCMAKE_BUILD_TYPE=Release",
	); err != nil {
		return err
	}
	if err := runner.run(ctx, "", "cmake", "--build", build, "--config", "Release"); err != nil {
		return err
	}
	if err := runner.run(ctx, "", "ctest", "--test-dir", build, "-C", "Release", "--output-on-failure"); err != nil {
		return err
	}

	return collectSnapshots(build, paths.destination, func(name string) bool {
		return strings.HasSuffix(name, "_cpp.sk")
	})
}
