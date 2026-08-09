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
	"path/filepath"
	"strings"
)

func generateGo(
	ctx context.Context,
	paths generationPaths,
	runner commandRunner,
) error {
	if err := runner.run(ctx, paths.source, "make", "generate-go-snapshots"); err != nil {
		return err
	}

	generated := filepath.Join(paths.source, "serialization_test_data", "go_generated_files")
	return collectSnapshots(generated, paths.destination, func(name string) bool {
		return strings.HasSuffix(name, "_go.sk")
	})
}
