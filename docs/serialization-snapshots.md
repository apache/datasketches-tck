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

This repository generates snapshots from the exact upstream commits recorded in `snapshot-revisions.toml` instead of following the latest branch. Pinning makes a checkout reproducible and ensures that changes to the compatibility corpus receive normal code review.

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

To update one source language, pass the upstream branch, tag, or commit to adopt:

```shell
mise run tck -- snapshots update go --revision main
```

The command resolves the supplied ref to an exact commit ID, generates the complete snapshot directory from that commit, and updates both `snapshot-revisions.toml` and `serialization/go/snapshots`. Generation happens before either file set is changed, so a generation failure leaves the repository untouched.

Choose a ref whose generator represents the compatibility behavior being adopted. If the upstream build or output layout changed, update the corresponding adapter in `internal/snapshots/<language>.go` before running the command.

Review the resolved pin and corpus together:

```shell
git diff --stat
git diff -- snapshot-revisions.toml serialization/go/snapshots
```

Added and deleted files change the set of compatibility cases. Unexpected changes to deterministic files should be understood from the upstream change before they are accepted.

Verify that generation is reproducible at the new pin and run the repository checks:

```shell
mise run tck -- snapshots check go
mise run check
```

A second generation may report allowed modifications for known probabilistic snapshots. The file set and deterministic contents must reproduce.

Because one revision identifies one upstream repository, `--revision` must target a single language. Use `all` without `--revision` only to regenerate every source implementation from its current pin.

To regenerate snapshots from the current pin without changing it, omit `--revision`.

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

For each requested language, the `tck` command reads the repository from `internal/snapshots/generator.go` and the commit from `snapshot-revisions.toml`, checks out that revision in a temporary workspace, and invokes the source-specific adapter in `internal/snapshots/<language>.go`. It then compares the generated output with `serialization/<language>/snapshots`; update mode atomically replaces that directory.

The command-line interface and change report live in `cmd/tck`. Reconciliation and file comparison live in `internal/snapshots`, where `stability.go` classifies deterministic and known probabilistic outputs.
