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

The serialization corpus is a compatibility boundary between DataSketches implementations. Each directory `serialization_test_data/<language>_generated_files` contains sketches produced by one implementation and intended to be read and validated by the others.

The DataSketches implementation repositories use the same `serialization_test_data/<language>_generated_files` layout for their cross-language fixtures, so a TCK checkout can serve directly as an implementation's test data root, and moving files between the TCK and an implementation is a plain copy.

The `main` branch of this repository publishes the latest self-tested snapshots from every source language. Each language validates its own snapshots with its generators and round-trip tests before they reach the TCK, so a snapshot set that passes there is ready to publish. Whether each implementation can read the others' snapshots is then answered by running its cross-language tests against the latest set, not by reviewing the `.sk` files by hand.

`config.toml` records the repository and exact commit each snapshot directory was generated from, so any TCK commit remains reproducible:

```toml
[snapshot.go]
repository = "https://github.com/apache/datasketches-go.git"
commit = "730c0ca31e00b8becf8b70591ae8ca73954912d0"
```

## Update to the latest snapshots

With the [toolchain installed](#set-up-the-toolchain), update every source to its default branch:

```shell
mise run tck -- snapshots update all
```

For each source, the command resolves the default branch to an exact commit ID, writes it to `config.toml`, and regenerates `serialization_test_data/<language>_generated_files`. Generation happens before either file set is changed, so a source whose generation fails is left untouched; the remaining sources are still updated and the command exits with an error naming the failed sources.

When a source's commit has not changed and only probabilistic snapshots differ, its snapshots are left as they are, so repeated updates do not add binary churn to the history.

The [Update snapshots](#github-actions) workflow runs this command weekly and commits the result to `main`.

## Update one source

Update a single source to its default branch, or to a specific branch, tag, or commit:

```shell
mise run tck -- snapshots update go
mise run tck -- snapshots update go v0.2.0
```

Pinning an older revision is useful for reproducing a release or for holding a source back while an upstream problem is fixed. If the upstream build or output layout changed, update the corresponding adapter in `internal/snapshots/<language>.go` before running the command.

## Review an update

Review the resolved pins and the corpus together:

```shell
git diff --stat
git diff -- config.toml serialization_test_data/go_generated_files
```

Most modified files are probabilistic snapshots, which are expected to change on every generation. The update report classifies each change, so the meaningful part of a review is short: added and deleted files change the set of compatibility cases, and changes to deterministic snapshots, such as a flag bit, should be understood from the upstream change.

Verify that generation is reproducible at the new pin and run the repository checks:

```shell
mise run tck -- snapshots check go
mise run check
```

A second generation may report allowed modifications for known probabilistic snapshots. The file set and deterministic contents must reproduce.

## Check the pinned corpus

Use check mode to regenerate snapshots from the currently pinned commit and compare them with the committed corpus:

```shell
mise run tck -- snapshots check go
```

Check mode does not modify the repository. It fails for added or deleted files and for content changes to deterministic snapshots. It reports, but allows, content changes to existing snapshots classified as probabilistic by `internal/snapshots/stability.go`.

This command answers whether the repository matches its pin; it does not determine whether the pin is the latest upstream commit. Use update mode for that.

## Synchronize configured snapshots

Regenerate every snapshot directory from the repositories and commits currently recorded in `config.toml`:

```shell
mise run tck -- snapshots sync
```

Sync may change the committed snapshot directories, but it never changes `config.toml`.

## Set up the toolchain

Install [mise](https://mise.jdx.dev/), then install the pinned toolchain and inspect the available snapshot commands:

```shell
mise install
mise run tck -- snapshots --help
```

Mise supplies Go, CMake and CTest, Java, and Maven. Git is required for every source language, a C++ compiler is required for C++, and Make is required for Go.

Check mode accepts `cpp`, `go`, `java`, or `all`. Synchronization takes no arguments. Update accepts `cpp`, `go`, `java`, or `all`, and an optional revision for a single source.

## Use the corpus from an implementation

An implementation consumes the `.sk` files as test fixtures. Its compatibility tests should load snapshots produced by the other source languages, deserialize each supported sketch family, and assert observable results with tolerances appropriate to that algorithm.

This repository centralizes the fixture corpus and source-side generation. Consumer tests remain in the individual DataSketches implementation repositories.

By default, an implementation should test against the latest snapshot set on `main`. A failure then shows exactly which source language and sketch disagree, as early as possible. To keep a release or a CI run from changing underneath it, an implementation can instead pin a specific TCK commit.

## Review policy for probabilistic snapshots

Some upstream generators contain randomness, so byte-for-byte reproduction is not a valid invariant for every file. The stability policy is source-specific because upstream implementations do not always seed or exercise an algorithm in the same way.

Only modifications to existing probabilistic files are allowed in check mode. Additions and deletions always block the check because they alter the compatibility corpus, and modifications to deterministic files block because they indicate either a compatibility change or a non-reproducible generator.

The probabilistic classification only controls byte-level comparison in this repository. Upstream generators remain responsible for constructing valid sketches, and consumers remain responsible for algorithm-appropriate assertions.

## GitHub Actions

`.github/workflows/check.yml` runs `mise run check` for pull requests and pushes to `main`. It validates the Go implementation of the TCK tooling, but it does not run the upstream snapshot generators or modify committed snapshots.

`.github/workflows/update-snapshots.yml` runs `snapshots update all` every Monday and on manual dispatch, and commits any updated pins and snapshots directly to `main`. It is not a required check. The job summary lists each source's revision change and change counts. If a source fails to generate, the other sources are still committed and the run fails so that the broken upstream is noticed.

## Implementation notes

For each requested language, the `tck` command reads the repository and commit from `config.toml`, checks out that revision in a temporary workspace, and invokes the source-specific adapter in `internal/snapshots/<language>.go`. It then compares the generated output with `serialization_test_data/<language>_generated_files`.

The command-line interface and change report live in `cmd/tck`. Reconciliation and file comparison live in `internal/snapshots`, where `stability.go` classifies deterministic and known probabilistic outputs.
