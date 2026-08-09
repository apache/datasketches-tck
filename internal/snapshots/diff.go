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
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type ChangeStatus string

const (
	ChangeAdded    ChangeStatus = "A"
	ChangeDeleted  ChangeStatus = "D"
	ChangeModified ChangeStatus = "M"
)

type FileMetadata struct {
	Size   int64
	SHA256 [sha256.Size]byte
}

type Change struct {
	Status    ChangeStatus
	Stability Stability
	Path      string
	Before    *FileMetadata
	After     *FileMetadata
}

type fileRecord struct {
	filename string
	metadata FileMetadata
}

func compareDirectories(
	current, generated string,
	stability func(string) Stability,
) ([]Change, error) {
	currentFiles, err := regularFiles(current)
	if err != nil {
		return nil, fmt.Errorf("inspect current snapshots: %w", err)
	}
	generatedFiles, err := regularFiles(generated)
	if err != nil {
		return nil, fmt.Errorf("inspect generated snapshots: %w", err)
	}

	paths := make(map[string]struct{}, len(currentFiles)+len(generatedFiles))
	for path := range currentFiles {
		paths[path] = struct{}{}
	}
	for path := range generatedFiles {
		paths[path] = struct{}{}
	}

	names := make([]string, 0, len(paths))
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)

	changes := make([]Change, 0)
	for _, name := range names {
		currentFile, inCurrent := currentFiles[name]
		generatedFile, inGenerated := generatedFiles[name]
		snapshotStability := stability(name)
		switch {
		case !inCurrent:
			after := generatedFile.metadata
			changes = append(changes, Change{
				Status: ChangeAdded, Stability: snapshotStability, Path: name, After: &after,
			})
		case !inGenerated:
			before := currentFile.metadata
			changes = append(changes, Change{
				Status: ChangeDeleted, Stability: snapshotStability, Path: name, Before: &before,
			})
		case currentFile.metadata != generatedFile.metadata:
			before := currentFile.metadata
			after := generatedFile.metadata
			changes = append(changes, Change{
				Status: ChangeModified, Stability: snapshotStability, Path: name, Before: &before, After: &after,
			})
		}
	}
	return changes, nil
}

func regularFiles(directory string) (map[string]fileRecord, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]fileRecord{}, nil
		}
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is not a directory", directory)
	}

	files := make(map[string]fileRecord)
	err = filepath.WalkDir(directory, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filename == directory || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link", filename)
		}

		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", filename)
		}

		relative, err := filepath.Rel(directory, filename)
		if err != nil {
			return err
		}
		metadata, err := fileMetadata(filename, entryInfo.Size())
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = fileRecord{filename: filename, metadata: metadata}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func fileMetadata(filename string, size int64) (FileMetadata, error) {
	file, err := os.Open(filename)
	if err != nil {
		return FileMetadata{}, err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return FileMetadata{}, err
	}

	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return FileMetadata{Size: size, SHA256: digest}, nil
}
