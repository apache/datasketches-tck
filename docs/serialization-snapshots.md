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

# Serialization snapshots

The serialization corpus is a compatibility boundary between DataSketches implementations. Each directory under `serialization/<language>/snapshots` contains sketches produced by one implementation and intended to be read and validated by the others.

This repository generates snapshots from exact upstream commits instead of following the latest branch. Pinning makes a checkout reproducible and ensures that changes to the compatibility corpus receive normal code review.

## Set up the toolchain

Install [mise](https://mise.jdx.dev/), then install the pinned toolchain and inspect the available snapshot commands:

```shell
mise install
mise run tck -- snapshots --help
```

Mise supplies Go, CMake and CTest, Java, and Maven. Git is required for every source language, a C++ compiler is required for C++, and Make is required for Go.

Commands accept `cpp`, `go`, `java`, or `all` as the source language.

## Update snapshots from upstream

Updating the corpus is an intentional maintainer operation because selecting an upstream revision and accepting compatibility changes require review. There is no GitHub Actions workflow that discovers newer revisions or opens snapshot update pull requests.

To update one source language:

1. Choose the upstream commit to adopt. Prefer a commit on the implementation's main development branch whose generator represents the compatibility behavior being adopted.
2. Change that language's `commit` field in `internal/snapshots/generator.go`. If the upstream build or output layout changed, update the corresponding adapter in `internal/snapshots/<language>.go` as well.
3. Regenerate the complete snapshot directory:

   ```shell
   mise run tck -- snapshots update go
   ```

   Update mode generates from the new pin and atomically replaces `serialization/go/snapshots`; it is not an incremental copy, so removed upstream outputs become visible as deletions.

4. Review the pin and corpus together:

   ```shell
   git diff --stat
   git diff -- internal/snapshots/generator.go
   git status --short serialization/go/snapshots
   ```

   Added and deleted files change the set of compatibility cases. Unexpected changes to deterministic files should be understood from the upstream change before they are accepted.

5. Verify that generation is reproducible at the new pin and run the repository checks:

   ```shell
   mise run tck -- snapshots check go
   mise run check
   ```

   A second generation may report allowed modifications for known probabilistic snapshots. The file set and deterministic contents must reproduce.

Use `all` instead of a language only when intentionally refreshing every source implementation. Updating languages separately usually produces smaller, easier-to-review pull requests.

## Check the pinned corpus

Use check mode to regenerate snapshots from the currently pinned commit and compare them with the committed corpus:

```shell
mise run tck -- snapshots check go
```

Check mode does not modify the repository. It fails for added or deleted files and for content changes to deterministic snapshots. It reports, but allows, content changes to existing snapshots classified as probabilistic by `internal/snapshots/stability.go`.

This command answers whether the repository matches its pin; it does not determine whether the pin is the latest upstream commit.

## Use the corpus from an implementation

An implementation consumes the `.sk` files as test fixtures. Its compatibility tests should load snapshots produced by the other source languages, deserialize each supported sketch family, and assert observable results with tolerances appropriate to that algorithm.

This repository centralizes the fixture corpus and source-side generation. Consumer tests remain in the individual DataSketches implementation repositories.

## Review policy for probabilistic snapshots

Some upstream generators contain randomness, so byte-for-byte reproduction is not a valid invariant for every file. The stability policy is source-specific because upstream implementations do not always seed or exercise an algorithm in the same way.

Only modifications to existing probabilistic files are allowed in check mode. Additions and deletions always block the check because they alter the compatibility corpus, and modifications to deterministic files block because they indicate either a compatibility change or a non-reproducible generator.

The probabilistic classification only controls byte-level comparison in this repository. Upstream generators remain responsible for constructing valid sketches, and consumers remain responsible for algorithm-appropriate assertions.

## GitHub Actions

`.github/workflows/check.yml` runs `mise run check` for pull requests and pushes to `main`. It validates the Go implementation of the TCK tooling, but it does not run the upstream snapshot generators or modify committed snapshots.

Snapshot generation is kept out of the required workflow because it clones and builds three external projects, and because an automated update cannot decide whether an upstream compatibility change should be adopted. A pull request that updates a pin should include the generated corpus changes and record which source-language checks were run locally.

## Implementation notes

For each requested language, the `tck` command reads the repository and commit from `internal/snapshots/generator.go`, checks out that revision in a temporary workspace, and invokes the source-specific adapter in `internal/snapshots/<language>.go`. It then compares the generated output with `serialization/<language>/snapshots`; update mode atomically replaces that directory.

The command-line interface and change report live in `cmd/tck`. Reconciliation and file comparison live in `internal/snapshots`, where `stability.go` classifies deterministic and known probabilistic outputs.
