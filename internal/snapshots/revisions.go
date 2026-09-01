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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

const revisionsFilename = "snapshot-revisions.toml"

const revisionsFileHeader = `# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements. See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License. You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Managed by: mise run tck -- snapshots update <language> --revision <ref>
# Values are resolved commit IDs; branches and tags must not be stored here.

`

var commitIDPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type revisionConfig struct {
	Revisions revisionSet `toml:"revisions"`
}

type revisionSet struct {
	CPP  string `toml:"cpp"`
	Go   string `toml:"go"`
	Java string `toml:"java"`
}

func loadRevisionConfig(repositoryRoot string) (revisionConfig, []byte, error) {
	filename := filepath.Join(repositoryRoot, revisionsFilename)
	info, err := os.Lstat(filename)
	if err != nil {
		return revisionConfig{}, nil, fmt.Errorf("inspect snapshot revisions: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return revisionConfig{}, nil, fmt.Errorf("snapshot revisions %s is not a regular file", filename)
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return revisionConfig{}, nil, fmt.Errorf("read snapshot revisions: %w", err)
	}

	var config revisionConfig
	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return revisionConfig{}, nil, fmt.Errorf("decode snapshot revisions: %w", err)
	}
	if err := config.validate(); err != nil {
		return revisionConfig{}, nil, err
	}
	return config, content, nil
}

func (config revisionConfig) validate() error {
	for _, language := range Languages() {
		revision, found := config.revision(language)
		if !found || !commitIDPattern.MatchString(revision) {
			return fmt.Errorf("snapshot revision for %s must be a 40-character lowercase commit ID", language)
		}
	}
	return nil
}

func (config revisionConfig) revision(language string) (string, bool) {
	switch language {
	case "cpp":
		return config.Revisions.CPP, true
	case "go":
		return config.Revisions.Go, true
	case "java":
		return config.Revisions.Java, true
	default:
		return "", false
	}
}

func (config *revisionConfig) setRevision(language, revision string) error {
	switch language {
	case "cpp":
		config.Revisions.CPP = revision
	case "go":
		config.Revisions.Go = revision
	case "java":
		config.Revisions.Java = revision
	default:
		return fmt.Errorf("unsupported snapshot language %q", language)
	}
	return config.validate()
}

func encodeRevisionConfig(config revisionConfig) ([]byte, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(
		"%s[revisions]\ncpp = %q\ngo = %q\njava = %q\n",
		revisionsFileHeader,
		config.Revisions.CPP,
		config.Revisions.Go,
		config.Revisions.Java,
	)), nil
}

func replaceRevisionFile(repositoryRoot string, content []byte) error {
	target := filepath.Join(repositoryRoot, revisionsFilename)
	info, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("inspect snapshot revisions before update: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("replace snapshot revisions: %s is not a regular file", target)
	}

	transaction, err := os.MkdirTemp(repositoryRoot, ".snapshot-revisions-")
	if err != nil {
		return fmt.Errorf("create revision update transaction: %w", err)
	}
	removeTransaction := true
	defer func() {
		if removeTransaction {
			_ = os.RemoveAll(transaction)
		}
	}()

	next := filepath.Join(transaction, "next")
	if err := os.WriteFile(next, content, 0o644); err != nil {
		return fmt.Errorf("stage snapshot revisions: %w", err)
	}

	previous := filepath.Join(transaction, "previous")
	if err := os.Rename(target, previous); err != nil {
		return fmt.Errorf("preserve current snapshot revisions: %w", err)
	}
	if err := os.Rename(next, target); err != nil {
		if restoreErr := os.Rename(previous, target); restoreErr != nil {
			removeTransaction = false
			return fmt.Errorf(
				"install snapshot revisions: %w; restoring previous revisions also failed: %v; previous file remains at %s",
				err,
				restoreErr,
				previous,
			)
		}
		return fmt.Errorf("install snapshot revisions: %w", err)
	}
	return nil
}
