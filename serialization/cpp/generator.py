#!/usr/bin/env python3

# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

import shutil

import tck
import typer


def main():
  print("--- Generating C++ Test Data ---")

  # 1. Check prerequisites
  tck.check_command_installed("git")
  tck.check_command_installed("cmake")
  tck.check_command_installed("ctest")

  # 2. Define paths
  repository_dir = tck.repository_root()
  temp_dir = repository_dir / "tmp_datasketches_cpp"
  output_dir = repository_dir / "serialization" / "cpp" / "snapshots"

  # 3. Setup temporary directory
  if temp_dir.exists():
    print(f"Removing existing temporary directory: {temp_dir}")
    shutil.rmtree(temp_dir)

  temp_dir.mkdir()

  # 4. Clone repository
  repo_url = "https://github.com/apache/datasketches-cpp.git"
  commit = "401423367055acdf7502e8ed3126730a08039d91"
  tck.run_command(
    [
      "git",
      "clone",
      "--depth",
      "1",
      "--revision",
      commit,
      "--single-branch",
      repo_url,
      str(temp_dir),
    ]
  )

  # 5. Build and Run CMake
  build_dir = temp_dir / "build"
  build_dir.mkdir(exist_ok=True)

  # Configure: Add CMAKE_BUILD_TYPE for single-config generators (Ninja/Make)
  tck.run_command(
    ["cmake", "..", "-DGENERATE=true", "-DCMAKE_BUILD_TYPE=Release"], cwd=build_dir
  )

  # Build: Release config
  tck.run_command(["cmake", "--build", ".", "--config", "Release"], cwd=build_dir)

  # Test: Use ctest which is more portable than 'cmake --target test' (VS uses RUN_TESTS)
  # --output-on-failure helps debug if a specific test fails
  tck.run_command(["ctest", "-C", "Release", "--output-on-failure"], cwd=build_dir)

  # 6. Copy generated files
  # The instructions say: cp datasketches-cpp/build/*/test/*_cpp.sk serialization_test_data/cpp_generated_files
  # We need to find where they are exactly.
  # It seems they might be in build/test/ or subdirectories depending on generator.

  print(f"Copying files to {output_dir}")
  output_dir.mkdir(parents=True, exist_ok=True)

  files_copied = 0

  for file_path in build_dir.rglob("*_cpp.sk"):
    shutil.copy2(file_path, output_dir)
    print(f"Copied: {file_path.name}")
    files_copied += 1

  if files_copied == 0:
    print("Warning: No *_cpp.sk files were found to copy.")
  else:
    print(f"Successfully copied {files_copied} files.")


if __name__ == "__main__":
  typer.run(main)
