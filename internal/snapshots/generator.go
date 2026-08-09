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
)

type generator struct {
	requirements []string
	generate     func(context.Context, string, string, commandRunner) error
	stability    func(string) Stability
}

var generators = map[string]generator{
	"cpp": {
		requirements: []string{"git", "cmake", "ctest"},
		generate:     generateCPP,
		stability:    cppStability,
	},
	"go": {
		requirements: []string{"git", "go", "make"},
		generate:     generateGo,
		stability:    goStability,
	},
	"java": {
		requirements: []string{"git", "java", "mvn"},
		generate:     generateJava,
		stability:    javaStability,
	},
}

func Languages() []string {
	return []string{"cpp", "go", "java"}
}

func HasLanguage(language string) bool {
	_, found := generators[language]
	return found
}

type commandRunner struct {
	stdout io.Writer
	stderr io.Writer
}
