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

import "testing"

func TestSnapshotStability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		stability func(string) Stability
		path      string
		expected  Stability
	}{
		{name: "cpp deterministic theta", stability: cppStability, path: "theta_n1000000_cpp.sk", expected: Stable},
		{name: "cpp bloom filter seed", stability: cppStability, path: "bf_n0_h3_cpp.sk", expected: Unstable},
		{name: "cpp exact KLL", stability: cppStability, path: "kll_long_n100_cpp.sk", expected: Stable},
		{name: "cpp compacted KLL", stability: cppStability, path: "kll_long_n1000_cpp.sk", expected: Unstable},
		{name: "cpp exact REQ", stability: cppStability, path: "req_float_n10_cpp.sk", expected: Stable},
		{name: "cpp compacted REQ", stability: cppStability, path: "req_float_n100_cpp.sk", expected: Unstable},
		{name: "java compacted quantiles", stability: javaStability, path: "quantiles_string_n1000_java.sk", expected: Unstable},
		{name: "java varopt exact mode", stability: javaStability, path: "varopt_sketch_long_n10_java.sk", expected: Stable},
		{name: "java varopt sampling mode", stability: javaStability, path: "varopt_sketch_long_n100_java.sk", expected: Unstable},
		{name: "go deterministic test KLL", stability: goStability, path: "kll_float_n1000000_go.sk", expected: Stable},
		{name: "go bloom filter seed", stability: goStability, path: "bf_byte_array_n10000_h3_go.sk", expected: Unstable},
		{name: "go reservoir exact mode", stability: goStability, path: "reservoir_items_long_exact_n128_k128_go.sk", expected: Stable},
		{name: "go reservoir sampling mode", stability: goStability, path: "reservoir_items_long_sampling_n1000_k128_go.sk", expected: Unstable},
		{name: "go compacted REQ", stability: goStability, path: "req_float_n100_go.sk", expected: Unstable},
		{name: "go varopt union sampling", stability: goStability, path: "varopt_union_double_sampling_go.sk", expected: Unstable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.stability(test.path); got != test.expected {
				t.Fatalf("%s classified as %s, want %s", test.path, got, test.expected)
			}
		})
	}
}
