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

## Update snapshots

With the [toolchain installed](#set-up-the-toolchain), update every available source to its remote default branch:

```shell
mise run tck -- snapshots update
```

Update is the only command that writes snapshots. It accepts an optional language and, after that, an optional revision:

| Command | Sources and revisions |
| --- | --- |
| `snapshots update` | All available languages at their remote `HEAD` |
| `snapshots update go` | Go at its remote `HEAD` |
| `snapshots update go HEAD` | The same explicit selection |
| `snapshots update go v0.2.0` | Go at the specified tag; branches and commit IDs also work |

A revision can only be supplied after a language. To regenerate or repair snapshots at an existing pin, pass the commit recorded in `config.toml` explicitly. Updating to an older revision follows the same process as updating to a newer one. If the upstream build or output layout changed, update the corresponding adapter in `internal/snapshots/<language>.go` before running the command.

Every selected source is generated in a temporary workspace and compared before any repository files are replaced. A generation or staging failure aborts the whole batch. Successful updates publish the selected snapshot directories and their exact commits in `config.toml` together. A changed revision with byte-identical snapshots only changes the pin, after generation has verified those contents.

### Skip unstable-only changes

`stable` snapshots have deterministic contents. `unstable` snapshots are known to vary between generations, even at the same source commit. By default, update adopts all generated differences, including unstable contents. Add `--skip-unstable-only` to any update invocation to avoid rewriting a source whose commit is unchanged and whose only differences are modifications to existing unstable files:

```shell
mise run tck -- snapshots update --skip-unstable-only
mise run tck -- snapshots update go --skip-unstable-only
mise run tck -- snapshots update go v0.2.0 --skip-unstable-only
```

The option applies independently to each selected language. It skips that source's entire update; it never filters individual files from an adopted snapshot set. A revision change, any added or deleted file, or any stable content change causes the complete generated snapshot set to be adopted, including its unstable files. The report identifies skipped sources and shows observed differences even when they were not written.

| Source revision | Snapshot differences | Default | With `--skip-unstable-only` |
| --- | --- | --- | --- |
| Unchanged | None | No changes | No changes |
| Unchanged | Only existing unstable contents | Replace snapshots | Keep the existing snapshots |
| Unchanged | Additions, deletions, or stable contents | Replace snapshots | Replace snapshots |
| Changed | None | Update the pin | Update the pin |
| Changed | Any | Update the pin and snapshots | Update the pin and snapshots |

### Publication and failures

The publisher stages the complete batch on the repository filesystem and retains backups until all replacements succeed. An installation error rolls back earlier replacements, including those for other languages. If rollback itself fails, the command reports the recovery directory and keeps its backups. Failure to remove backups after a successful publication is a warning and does not undo the published pins or snapshots.

This is rollback protection for ordinary errors, not a crash-atomic filesystem transaction or synchronization for concurrent local writers. Do not run multiple updates against the same checkout. The command refuses to overwrite config edits detected after generation. In CI, an update failure prevents the commit step; a successful batch is published in one Git commit and a normal push. A rejected push leaves the remote unchanged; a push publishes the complete commit, never part of the batch.

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

## Set up the toolchain

Install [mise](https://mise.jdx.dev/), then install the pinned toolchain and inspect the available snapshot commands:

```shell
mise install
mise run tck -- snapshots --help
```

Mise supplies Go, CMake and CTest, Java, and Maven. Git is required for every source language, a C++ compiler is required for C++, and Make is required for Go.

Check accepts `cpp`, `go`, `java`, or `all` and is always read-only. Update defaults to all languages at `HEAD`; specifying a language limits the update to that source, and a second argument selects its revision. The former `snapshots sync` command is replaced by `snapshots update <language> <recorded-commit>`.

## Use the corpus from an implementation

An implementation consumes the `.sk` files as test fixtures. Its compatibility tests should load snapshots produced by the other source languages, deserialize each supported sketch family, and assert observable results with tolerances appropriate to that algorithm.

This repository centralizes the fixture corpus and source-side generation. Consumer tests remain in the individual DataSketches implementation repositories.

By default, an implementation should test against the latest snapshot set on `main`. A failure then shows exactly which source language and sketch disagree, as early as possible. To keep a release or a CI run from changing underneath it, an implementation can instead pin a specific TCK commit.

## Review policy for probabilistic snapshots

Some upstream generators contain randomness, so byte-for-byte reproduction is not a valid invariant for every file. The stability policy is source-specific because upstream implementations do not always seed or exercise an algorithm in the same way.

Only modifications to existing probabilistic files are allowed in check mode. Additions and deletions always block the check because they alter the compatibility corpus, and modifications to deterministic files block because they indicate either a compatibility change or a non-reproducible generator.

The stable/unstable classification controls check results and the optional unstable-only update policy. It does not prove that an individual difference is random or that an old snapshot is valid for a new source revision. Upstream generators remain responsible for constructing valid sketches, and consumers remain responsible for algorithm-appropriate assertions.

## GitHub Actions

`.github/workflows/check.yml` runs `mise run check` for pull requests and pushes to `main`. It validates the Go implementation of the TCK tooling, but it does not run the upstream snapshot generators or modify committed snapshots.

`.github/workflows/update-snapshots.yml` runs `snapshots update --skip-unstable-only` every Monday and on manual dispatch. It is not a required check. The job summary lists each source's revision change, change counts, and skipped updates. The workflow commits and pushes the batch only after the command succeeds; any failed source prevents publication of the whole batch.

## Implementation notes

For each requested language, the `tck` command reads the source repository and existing pin from `config.toml`. Check generates at that pin; update resolves the requested revision, defaulting to remote `HEAD`. Both invoke the source-specific adapter in `internal/snapshots/<language>.go` in a temporary workspace and compare its output with `serialization_test_data/<language>_generated_files`. Update stages and publishes the selected directories and revised config as one batch.

The command-line interface and change report live in `cmd/tck`. Reconciliation and file comparison live in `internal/snapshots`, where `stability.go` classifies deterministic and known probabilistic outputs.
