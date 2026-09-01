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

## How generation works

The repository-local `tck` command performs the following steps for each requested source language:

1. It reads the upstream repository and commit from `internal/snapshots/generator.go`.
2. It creates a temporary workspace and checks out that exact commit with a shallow fetch.
3. It runs the upstream implementation's own snapshot generator through the adapter in `internal/snapshots/<language>.go`.
4. It compares the generated files with `serialization/<language>/snapshots` and reports additions, modifications, and deletions.
5. In update mode, it atomically replaces the selected committed snapshot directory with the generated directory.

The main components are:

| Path | Responsibility |
| --- | --- |
| `cmd/tck` | Command-line interface and human-readable change report |
| `internal/snapshots/generator.go` | Upstream repositories, pinned commits, tool requirements, and generator registry |
| `internal/snapshots/{cpp,go,java}.go` | Source-specific build and collection adapters |
| `internal/snapshots/reconcile.go` | Check and atomic update orchestration |
| `internal/snapshots/{diff,files}.go` | File comparison and copying |
| `internal/snapshots/stability.go` | Classification of deterministic and known probabilistic outputs |
| `serialization/<language>/snapshots` | The committed compatibility corpus |

## Use the corpus from an implementation

An implementation consumes the `.sk` files as test fixtures. Its compatibility tests should load snapshots produced by the other source languages, deserialize each supported sketch family, and assert the sketch's observable results with tolerances appropriate to that algorithm.

This repository currently centralizes the fixture corpus and the source-side generation process. It does not invoke the consumer libraries or define a shared assertion runner; those tests remain in the individual DataSketches implementation repositories.

## Set up the toolchain

Install [mise](https://mise.jdx.dev/) and let it install the versions pinned in `mise.toml`:

```shell
mise install
```

Mise supplies Go, CMake and CTest, Java, and Maven. Git is required for every source language, a C++ compiler is required for C++, and Make is required for Go.

To inspect the available commands without generating anything, run:

```shell
mise run tck -- snapshots --help
```

Commands accept `cpp`, `go`, `java`, or `all` as the source language.

## Check the pinned corpus

Use check mode to regenerate snapshots from the currently pinned commit and compare them with the committed corpus:

```shell
mise run tck -- snapshots check go
```

Check mode does not modify the repository. It fails for added or deleted files and for content changes to deterministic snapshots. It reports, but allows, content changes to existing snapshots classified as probabilistic by `internal/snapshots/stability.go`.

This command answers whether the repository matches its pin; it does not determine whether the pin is the latest upstream commit.

## Update snapshots from upstream

Updating the corpus is an intentional maintainer operation because selecting an upstream revision and accepting compatibility changes require review. There is currently no GitHub Actions workflow that discovers newer upstream revisions or opens snapshot update pull requests.

To update one source language:

1. Choose the upstream commit to adopt. Prefer a commit on the implementation's main development branch whose snapshot generator represents the compatibility behavior being adopted.
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

   Pay particular attention to added and deleted files because they change the set of cross-language compatibility cases. For deterministic files, unexpected byte changes should be understood from the upstream change before they are accepted.

5. Verify that generation is reproducible at the new pin and run the repository checks:

   ```shell
   mise run tck -- snapshots check go
   mise run check
   ```

   A second generation may report allowed modifications for known probabilistic snapshots. The important conditions are that the file set is unchanged and deterministic contents reproduce.

Use `all` instead of a language only when intentionally refreshing every source implementation. Updating languages separately usually produces smaller, easier-to-review pull requests.

## Review policy for probabilistic snapshots

Some upstream generators contain randomness, so byte-for-byte reproduction is not a valid invariant for every file. The stability policy is source-specific because upstream implementations do not always seed or exercise an algorithm in the same way.

Only modifications to existing probabilistic files are allowed in check mode. Additions and deletions always block the check because they alter the compatibility corpus, and modifications to deterministic files block because they indicate either a compatibility change or a non-reproducible generator.

The upstream generation suite is still responsible for constructing valid sketches. Consumers of this repository remain responsible for deserializing the files and asserting algorithm-appropriate behavior; the probabilistic classification only controls byte-level comparison in this repository.

## GitHub Actions

`.github/workflows/check.yml` runs `mise run check` for pull requests and pushes to `main`. It validates the Go implementation of the TCK tooling, but it does not run the upstream snapshot generators or modify committed snapshots.

Snapshot generation is kept out of the required workflow because it clones and builds three external projects, and because an automated update cannot decide whether an upstream compatibility change should be adopted. A pull request that updates a pin should include the generated corpus changes and record which source-language checks were run locally.
