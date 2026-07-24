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
import sys
import subprocess
from pathlib import Path

def repository_root():
    """Returns the root directory of the repository."""
    return Path(__file__).resolve().parents[2]


def check_command_installed(command):
    """Checks if a command is available in the system path."""
    if shutil.which(command) is None:
        print(f"Error: '{command}' is not installed or not in PATH.")
        sys.exit(1)


def run_command(command, cwd=None, shell=False):
    """Runs a shell command, streaming output to stdout/stderr."""
    cmd_str = ' '.join(command) if isinstance(command, list) else command
    print(f"Running: {cmd_str}")
    sys.stdout.flush() # Ensure 'Running' message appears before command output
    try:
        # Don't capture output; let it stream to sys.stdout/sys.stderr
        subprocess.check_call(command, cwd=cwd, stderr=subprocess.STDOUT, shell=shell)
    except subprocess.CalledProcessError as e:
        print(f"Error running command: {e}")
        print("--- OUTPUT ---")
        print(e.stdout)
        print("--- END OUTPUT ---")
        sys.exit(1)
