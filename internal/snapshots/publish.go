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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

type replacement struct {
	target string
	source string
}

// publish stages the complete batch on the destination filesystem, then keeps
// backups until every replacement succeeds. Rollback failures preserve those
// backups for recovery. Cleanup after success must never trigger a rollback.
// This protects ordinary error paths, not concurrent readers or process crashes.
func publish(root string, replacements []replacement, stderr io.Writer) (err error) {
	if len(replacements) == 0 {
		return nil
	}
	transaction, err := os.MkdirTemp(root, ".tck-update-")
	if err != nil {
		return fmt.Errorf("create update transaction: %w", err)
	}
	var undo []func() error
	defer func() {
		if err != nil {
			var rollbackErr error
			for index := len(undo) - 1; index >= 0; index-- {
				rollbackErr = errors.Join(rollbackErr, undo[index]())
			}
			if rollbackErr != nil {
				err = fmt.Errorf("%w; rollback failed: %v; recovery files remain at %s", err, rollbackErr, transaction)
				return
			}
		}
		if cleanupErr := os.RemoveAll(transaction); cleanupErr != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: could not remove update backups at %s: %v\n", transaction, cleanupErr)
		}
	}()

	for index, replacement := range replacements {
		next := filepath.Join(transaction, "next-"+strconv.Itoa(index))
		info, err := os.Stat(replacement.source)
		if err != nil {
			return err
		}
		if current, err := os.Lstat(replacement.target); err == nil {
			if current.Mode()&os.ModeSymlink != 0 || current.IsDir() != info.IsDir() || (!current.IsDir() && !current.Mode().IsRegular()) {
				return fmt.Errorf("cannot replace %s: unexpected file type", replacement.target)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if info.IsDir() {
			err = copyDirectory(replacement.source, next)
		} else {
			err = copyFile(replacement.source, next)
		}
		if err != nil {
			return fmt.Errorf("stage %s: %w", replacement.target, err)
		}
	}
	for index, replacement := range replacements {
		target := replacement.target
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		previous := filepath.Join(transaction, "previous-"+strconv.Itoa(index))
		if err := os.Rename(target, previous); err == nil {
			undo = append(undo, func() error { return os.Rename(previous, target) })
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("preserve %s: %w", target, err)
		}
		next := filepath.Join(transaction, "next-"+strconv.Itoa(index))
		if err := os.Rename(next, target); err != nil {
			return fmt.Errorf("install %s: %w", target, err)
		}
		undo = append(undo, func() error { return os.RemoveAll(target) })
	}
	return nil
}

func copyDirectory(source, destination string) error {
	files, err := regularFiles(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	for relative, file := range files {
		output := filepath.Join(destination, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		if err := copyFile(file.filename, output); err != nil {
			return err
		}
	}
	return nil
}
