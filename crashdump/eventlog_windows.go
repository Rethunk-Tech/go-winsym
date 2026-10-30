package crashdump

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: a GUI process has no console, so Windows opens one for every console program it
// starts unless told not to. Unlike hiding the window, it is honoured when Windows Terminal is the default console.
const createNoWindow = 0x08000000

func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
