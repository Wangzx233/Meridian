package control

import "strings"

func serverDisplayName(server Server) string {
	if server.Alias != nil {
		alias := strings.TrimSpace(*server.Alias)
		if alias != "" {
			return alias
		}
	}
	return server.Name
}
