# Apache® DataSketches™ Technology Compatibility Kit

This repository provides a Technology Compatibility Kit for the DataSketches library.

A Technology Compatibility Kit (TCK) a suite of tests that at least nominally checks a particular DataSketches implementation for compliance.

## Overview

### Serialization snapshots

The structure (or image) of a serialized sketch is independent of the language from which it was created.

This repository contains snapshots of serialized sketches, which a particular DataSketches implementation should be able to read. Snapshot generators are also included.

### Snapshot generators

The repository-local Go CLI checks out pinned DataSketches implementations and runs their own snapshot generation suites. [mise](https://mise.jdx.dev/) installs the required Go, CMake, Java, and Maven toolchains and exposes the CLI:

```shell
mise install
mise run tck -- snapshots check go
mise run tck -- snapshots update go
```

Replace `go` with `cpp`, `java`, or `all`. Git is required for every generator. The C++ generator additionally requires CTest and a C++ compiler, while the Go generator requires Make.

`check` leaves the repository unchanged. It reports added, modified, and deleted files with their stability class, size, and SHA-256 digest. Added or deleted files and modifications to stable snapshots make the check fail. Modifications to known probabilistic snapshots are reported but allowed. `update` atomically replaces the selected snapshot directory with freshly generated files.

The upstream repositories and exact commits are pinned together in `internal/snapshots/revisions.go`. The stability rules reflect repeated generation at those commits and the project discussion about probabilistic sketches:

| Source | Snapshots allowed to vary |
| --- | --- |
| C++ and Java | Bloom filters; KLL and classic quantiles after compaction (`n >= 1000`); REQ after compaction (`n >= 100`); VarOpt sampling mode |
| Go | Bloom filters; REQ after compaction (`n >= 100`); reservoir and VarOpt sampling mode |

The pinned Go KLL generator enables its deterministic test offset, so its generated KLL files remain strictly checked. These rules do not claim that probabilistic snapshots are logically unchecked: the upstream generation suites validate their construction, while consuming DataSketches implementations remain responsible for compatibility assertions with algorithm-appropriate tolerances.

Background: [central snapshot generation issue](https://github.com/apache/datasketches-rust/issues/10#issuecomment-3663796398), [cross-language testing discussion](https://github.com/apache/datasketches-rust/discussions/4#discussioncomment-15226056), [dev mailing-list inventory](https://www.mail-archive.com/dev%40datasketches.apache.org/msg04302.html), and [KLL non-determinism discussion](https://github.com/apache/datasketches-java/issues/693).

## Contribute

Please visit the main [DataSketches website](https://datasketches.apache.org) for more information.

If you are interested in making contributions to this site, please see our [Community](https://datasketches.apache.org/docs/Community/) page for how to contact us.

## License

Licensed under the Apache License, Version 2.0: http://www.apache.org/licenses/LICENSE-2.0
