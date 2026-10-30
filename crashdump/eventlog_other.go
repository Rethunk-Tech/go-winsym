//go:build !windows

package crashdump

import "os/exec"

func hideWindow(*exec.Cmd) {}
