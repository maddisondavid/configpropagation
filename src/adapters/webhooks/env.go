package webhooks

import "strings"

// parseBoolEnv interprets common truthy values (1, true, yes, on) in a
// case-insensitive manner.
func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
