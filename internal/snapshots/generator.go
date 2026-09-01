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
	"io"
	"path/filepath"
	"sort"
)

type generationPaths struct {
	workspace   string
	source      string
	destination string
}

type generateFunc func(context.Context, generationPaths, commandRunner) error

type generator struct {
	repository   string
	requirements []string
	generate     generateFunc
	stability    func(string) Stability
}

var generators = map[string]generator{
	"cpp": {
		repository:   "https://github.com/apache/datasketches-cpp.git",
		requirements: []string{"git", "cmake", "ctest"},
		generate:     generateCPP,
		stability:    cppStability,
	},
	"go": {
		repository:   "https://github.com/apache/datasketches-go.git",
		requirements: []string{"git", "go", "make"},
		generate:     generateGo,
		stability:    goStability,
	},
	"java": {
		repository:   "https://github.com/apache/datasketches-java.git",
		requirements: []string{"git", "java", "mvn"},
		generate:     generateJava,
		stability:    javaStability,
	},
}

func Languages() []string {
	languages := make([]string, 0, len(generators))
	for language := range generators {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	return languages
}

func (definition generator) run(
	ctx context.Context,
	workspace, destination, revision string,
	runner commandRunner,
) (string, error) {
	paths := generationPaths{
		workspace:   workspace,
		source:      filepath.Join(workspace, "source"),
		destination: destination,
	}
	resolvedRevision, err := cloneAtRevision(ctx, runner, definition.repository, revision, paths.source)
	if err != nil {
		return "", err
	}
	if err := definition.generate(ctx, paths, runner); err != nil {
		return "", err
	}
	return resolvedRevision, nil
}

type commandRunner struct {
	stdout io.Writer
	stderr io.Writer
}
