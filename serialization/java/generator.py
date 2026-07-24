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

import os
import shutil
import sys

import tck
import typer


def main():
    print("--- Generating Java Test Data ---")

    # 1. Check prerequisites
    tck.check_command_installed("git")
    tck.check_command_installed("java")
    mvn_cmd_name = "mvn"
    if os.name == 'nt':
        mvn_cmd_name = "mvn.cmd"
    tck.check_command_installed(mvn_cmd_name)

    # 2. Define paths
    repository_dir = tck.repository_root()
    temp_dir = repository_dir / "tmp_datasketches_java"
    output_dir = repository_dir / "serialization" / "java" / "snapshots"

    # 3. Setup temporary directory
    if temp_dir.exists():
        print(f"Removing existing temporary directory: {temp_dir}")
        shutil.rmtree(temp_dir)

    temp_dir.mkdir()

    # 4. Clone repository
    repo_url = "https://github.com/apache/datasketches-java.git"
    branch = "9.0.0"
    tck.run_command([
        "git", "clone",
        "--depth", "1",
        "--branch", branch,
        "--single-branch",
        repo_url,
        str(temp_dir)
    ])

    # 5. Run Maven to generate files
    mvn_cmd = [mvn_cmd_name, "test", "-P", "generate-java-files"]
    use_shell = os.name == 'nt' # Windows
    tck.run_command(mvn_cmd, cwd=temp_dir, shell=use_shell)

    # 6. Copy generated files
    generated_files_dir = temp_dir / "serialization_test_data" / "java_generated_files"

    if not generated_files_dir.exists():
        print(f"Error: Expected generated files directory not found at {generated_files_dir}")
        sys.exit(1)

    print(f"Copying files from {generated_files_dir} to {output_dir}")
    output_dir.mkdir(parents=True, exist_ok=True)

    files_copied = 0
    for file_path in generated_files_dir.glob("*.sk"):
        shutil.copy2(file_path, output_dir)
        print(f"Copied: {file_path.name}")
        files_copied += 1

    if files_copied == 0:
        print("Warning: No .sk files were found to copy.")
    else:
        print(f"Successfully copied {files_copied} files.")


if __name__ == "__main__":
    typer.run(main)
