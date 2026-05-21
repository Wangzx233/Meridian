//go:build windows

package agent

import "errors"

func execCurrentProcess(path string, argv []string, env []string) error {
	return errors.New("exec self-update is not supported on Windows")
}
