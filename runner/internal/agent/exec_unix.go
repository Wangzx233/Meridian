//go:build !windows

package agent

import "syscall"

func execCurrentProcess(path string, argv []string, env []string) error {
	return syscall.Exec(path, argv, env)
}
