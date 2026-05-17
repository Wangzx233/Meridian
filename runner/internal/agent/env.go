package agent

import (
	"os"
	"strings"
)

type mergedEnvList []string

func mergedEnv(extra []string) []string {
	env := mergedEnvList(os.Environ())
	if len(extra) == 0 {
		return env
	}
	for _, entry := range extra {
		env = env.with(entry)
	}
	return env
}

func (e mergedEnvList) with(entry string) mergedEnvList {
	key, _, ok := strings.Cut(entry, "=")
	if !ok || key == "" {
		return e
	}
	for i, current := range e {
		currentKey, _, currentOK := strings.Cut(current, "=")
		if currentOK && strings.EqualFold(currentKey, key) {
			e[i] = entry
			return e
		}
	}
	return append(e, entry)
}
