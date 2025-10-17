package webhooks

import "testing"

func TestParseBoolEnv(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", " yes ", "On"}
	for _, v := range truthy {
		if !parseBoolEnv(v) {
			t.Fatalf("expected %q to be truthy", v)
		}
	}

	falsy := []string{"0", "false", "", "no", "off", "maybe"}
	for _, v := range falsy {
		if parseBoolEnv(v) {
			t.Fatalf("expected %q to be falsy", v)
		}
	}
}
