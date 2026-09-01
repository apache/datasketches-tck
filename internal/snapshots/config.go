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

const configFilename = "config.toml"

const configFileHeader = `# Licensed to the Apache Software Foundation (ASF) under one or more
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

`

var commitIDPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type repositoryConfig struct {
	Snapshot snapshotSources `toml:"snapshot"`
}

type snapshotSources struct {
	CPP  snapshotSource `toml:"cpp"`
	Go   snapshotSource `toml:"go"`
	Java snapshotSource `toml:"java"`
}

type snapshotSource struct {
	Repository string `toml:"repository"`
	Commit     string `toml:"commit"`
}

func loadConfig(repositoryRoot string) (repositoryConfig, []byte, error) {
	filename := filepath.Join(repositoryRoot, configFilename)
	content, err := os.ReadFile(filename)
	if err != nil {
		return repositoryConfig{}, nil, fmt.Errorf("read TCK config: %w", err)
	}

	var config repositoryConfig
	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return repositoryConfig{}, nil, fmt.Errorf("decode TCK config: %w", err)
	}
	if err := config.validate(); err != nil {
		return repositoryConfig{}, nil, err
	}
	return config, content, nil
}

func (config repositoryConfig) validate() error {
	for _, language := range Languages() {
		source, found := config.source(language)
		if !found {
			return fmt.Errorf("snapshot source for %s is not configured", language)
		}
		if source.Repository == "" {
			return fmt.Errorf("snapshot repository for %s must not be empty", language)
		}
		if !commitIDPattern.MatchString(source.Commit) {
			return fmt.Errorf("snapshot commit for %s must be a 40-character lowercase commit ID", language)
		}
	}
	return nil
}

func (config repositoryConfig) source(language string) (snapshotSource, bool) {
	switch language {
	case "cpp":
		return config.Snapshot.CPP, true
	case "go":
		return config.Snapshot.Go, true
	case "java":
		return config.Snapshot.Java, true
	default:
		return snapshotSource{}, false
	}
}

func (config *repositoryConfig) setCommit(language, commit string) error {
	switch language {
	case "cpp":
		config.Snapshot.CPP.Commit = commit
	case "go":
		config.Snapshot.Go.Commit = commit
	case "java":
		config.Snapshot.Java.Commit = commit
	default:
		return fmt.Errorf("unsupported snapshot language %q", language)
	}
	return config.validate()
}

func encodeConfig(config repositoryConfig) ([]byte, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(
		"%s[snapshot.cpp]\nrepository = %q\ncommit = %q\n\n"+
			"[snapshot.go]\nrepository = %q\ncommit = %q\n\n"+
			"[snapshot.java]\nrepository = %q\ncommit = %q\n",
		configFileHeader,
		config.Snapshot.CPP.Repository,
		config.Snapshot.CPP.Commit,
		config.Snapshot.Go.Repository,
		config.Snapshot.Go.Commit,
		config.Snapshot.Java.Repository,
		config.Snapshot.Java.Commit,
	)), nil
}

func replaceConfigFile(repositoryRoot string, content []byte) error {
	target := filepath.Join(repositoryRoot, configFilename)
	transaction, err := os.MkdirTemp(repositoryRoot, ".tck-config-")
	if err != nil {
		return fmt.Errorf("create config update transaction: %w", err)
	}
	defer func() { _ = os.RemoveAll(transaction) }()

	next := filepath.Join(transaction, "next")
	if err := os.WriteFile(next, content, 0o644); err != nil {
		return fmt.Errorf("stage TCK config: %w", err)
	}
	previous := filepath.Join(transaction, "previous")
	if err := os.Rename(target, previous); err != nil {
		return fmt.Errorf("preserve current TCK config: %w", err)
	}
	if err := os.Rename(next, target); err != nil {
		_ = os.Rename(previous, target)
		return fmt.Errorf("install TCK config: %w", err)
	}
	return nil
}
