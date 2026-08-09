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
	"html"
	"os"
	"path/filepath"
	"strings"
)

func generateJava(ctx context.Context, workspace, destination string, runner commandRunner) error {
	source := filepath.Join(workspace, "source")
	if err := cloneAtCommit(ctx, runner, javaRepository, javaCommit, source); err != nil {
		return err
	}

	toolchains, err := writeMavenToolchains(workspace)
	if err != nil {
		return err
	}
	if err := runner.run(
		ctx,
		source,
		"mvn",
		"--toolchains", toolchains,
		"test",
		"-P", "generate-java-files",
	); err != nil {
		return err
	}

	generated := filepath.Join(source, "serialization_test_data", "java_generated_files")
	return collectSnapshots(generated, destination, func(name string) bool {
		return strings.HasSuffix(name, "_java.sk")
	})
}

func writeMavenToolchains(workspace string) (string, error) {
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome == "" {
		return "", fmt.Errorf("JAVA_HOME must point to a JDK 25 installation; run the CLI through mise")
	}
	info, err := os.Stat(javaHome)
	if err != nil {
		return "", fmt.Errorf("inspect JAVA_HOME: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("JAVA_HOME %s is not a directory", javaHome)
	}

	filename := filepath.Join(workspace, "maven-toolchains.xml")
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<toolchains>
  <toolchain>
    <type>jdk</type>
    <provides>
      <version>25</version>
    </provides>
    <configuration>
      <jdkHome>%s</jdkHome>
    </configuration>
  </toolchain>
</toolchains>
`, html.EscapeString(javaHome))
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write temporary Maven toolchains file: %w", err)
	}
	return filename, nil
}
