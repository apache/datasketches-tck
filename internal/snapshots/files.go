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
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func collectSnapshots(source, destination string, include func(string) bool) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("inspect generated snapshots: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("generated snapshot path %s is not a directory", source)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create snapshot staging directory: %w", err)
	}

	seen := make(map[string]string)
	err = filepath.WalkDir(source, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("generated file %s is a symbolic link", filename)
		}
		if !include(entry.Name()) {
			return nil
		}

		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("generated file %s is not regular", filename)
		}
		if previous, found := seen[entry.Name()]; found {
			return fmt.Errorf("duplicate generated snapshot %s in %s and %s", entry.Name(), previous, filename)
		}
		seen[entry.Name()] = filename
		return copyFile(filename, filepath.Join(destination, entry.Name()))
	})
	if err != nil {
		return fmt.Errorf("collect generated snapshots: %w", err)
	}
	if len(seen) == 0 {
		return fmt.Errorf("no snapshots found under %s", source)
	}
	return nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()

	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}
