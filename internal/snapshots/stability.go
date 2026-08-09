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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Stability string

const (
	Stable   Stability = "stable"
	Unstable Stability = "unstable"
)

var countPattern = regexp.MustCompile(`_n([0-9]+)(?:_|$)`)

// The classifications below follow the project discussion at
// https://github.com/apache/datasketches-rust/issues/10#issuecomment-3663796398
// and the repeated-generation inventory recorded at
// https://www.mail-archive.com/dev%40datasketches.apache.org/msg04302.html.
// They are intentionally source-specific: for example, the pinned Go KLL
// generator enables its deterministic test offset, while C++ and Java do not.

func cppStability(path string) Stability {
	if cppOrJavaUnstable(filepath.Base(path)) {
		return Unstable
	}
	return Stable
}

func javaStability(path string) Stability {
	if cppOrJavaUnstable(filepath.Base(path)) {
		return Unstable
	}
	return Stable
}

func goStability(path string) Stability {
	name := filepath.Base(path)
	switch {
	case strings.HasPrefix(name, "bf_"):
		return Unstable
	case strings.HasPrefix(name, "req_float_") && countAtLeast(name, 100):
		return Unstable
	case strings.HasPrefix(name, "reservoir_") && strings.Contains(name, "_sampling_"):
		return Unstable
	case varOptUnstable(name):
		return Unstable
	default:
		return Stable
	}
}

func cppOrJavaUnstable(name string) bool {
	switch {
	case strings.HasPrefix(name, "bf_"):
		return true
	case (strings.HasPrefix(name, "kll_") || strings.HasPrefix(name, "quantiles_")) && countAtLeast(name, 1000):
		return true
	case strings.HasPrefix(name, "req_float_") && countAtLeast(name, 100):
		return true
	case varOptUnstable(name):
		return true
	default:
		return false
	}
}

func varOptUnstable(name string) bool {
	if !strings.HasPrefix(name, "varopt_") {
		return false
	}
	return strings.Contains(name, "_sampling_") || countAtLeast(name, 100)
}

func countAtLeast(name string, minimum int) bool {
	match := countPattern.FindStringSubmatch(name)
	if len(match) != 2 {
		return false
	}
	count, err := strconv.Atoi(match[1])
	return err == nil && count >= minimum
}
