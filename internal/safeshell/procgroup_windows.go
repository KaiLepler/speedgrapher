// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows

package safeshell

import "os/exec"

// configureProcessGroup is a no-op on Windows. POSIX process groups don't
// apply here; WaitDelay (set alongside cmd.Cancel) still bounds how long a
// timed-out command can block Wait, even though descendant processes are
// not explicitly terminated.
func configureProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup falls back to killing the direct child process only.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
