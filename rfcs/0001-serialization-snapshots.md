<!--
  Licensed to the Apache Software Foundation (ASF) under one or more
  contributor license agreements. See the NOTICE file distributed with
  this work for additional information regarding copyright ownership.
  The ASF licenses this file to You under the Apache License, Version 2.0
  (the "License"); you may not use this file except in compliance with
  the License. You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
-->

# RFC 0001: Serialization snapshots

[PR #1](https://github.com/apache/datasketches-tck/pull/1) introduced the snapshot corpus and its generators so that DataSketches implementations can test whether they read each other's serialized sketches.

## Publishing snapshots

Each directory in [`serialization_test_data/`](../serialization_test_data/) contains `.sk` files produced by one implementation. The TCK and the implementation repositories share the `serialization_test_data/<language>_generated_files` layout introduced in [PR #14](https://github.com/apache/datasketches-tck/pull/14), so a TCK checkout can serve directly as an implementation's test data root.

Each source implementation validates its own snapshots with its generators and round-trip tests. The TCK publishes the resulting files on `main` and records the upstream repository and exact source commit for each language in [`config.toml`](../config.toml). The [snapshot maintenance guide](../docs/serialization-snapshots.md) describes how to update the corpus and check it against those pins.

## Testing an implementation

Cross-language tests live in the consuming implementation. They load snapshots produced by the other languages, deserialize each supported sketch family, and assert observable results with tolerances appropriate to that algorithm.

By default, an implementation tests against the latest corpus on `main`, a policy adopted in [PR #15](https://github.com/apache/datasketches-tck/pull/15). A failure then identifies which source language and sketch disagree with the reader. An implementation can pin a TCK commit to run a release or CI build against a fixed set of fixtures.
