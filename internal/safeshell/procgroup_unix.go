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

//go:build !windows

package safeshell

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup places cmd in a new process group so that, on
// cancellation, killProcessGroup can terminate the entire subprocess tree
// (including any children the command itself spawns) rather than just the
// direct child process.
func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup sends SIGKILL to the whole process group led by cmd's
// process, ensuring descendants (e.g. a subprocess forked by a shell script)
// are terminated too, instead of being orphaned and left running.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// A negative pid signals every process in the group whose id equals
	// the absolute value, which is the pid of the group leader (cmd's
	// direct child, since Setpgid was set before Start).
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err != nil {
		// Fall back to killing at least the direct child if the group
		// kill itself failed (e.g. the group has already exited).
		_ = cmd.Process.Kill()
	}
	return nil
}
