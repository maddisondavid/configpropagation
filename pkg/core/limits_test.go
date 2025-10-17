package core

import "testing"

func TestCheckConfigMapSize(t *testing.T) {
	data := map[string]string{"a": "12345", "b": "x"}
	res := CheckConfigMapSize(data)
	wantBytes := len("a") + len("12345") + len("b") + len("x")
	if res.Bytes != wantBytes {
		t.Fatalf("expected %d bytes got %d", wantBytes, res.Bytes)
	}
	if res.Warn || res.Block {
		t.Fatalf("unexpected flags: %+v", res)
	}

	big := map[string]string{"a": string(make([]byte, ConfigMapSizeWarnThresholdBytes+1))}
	warn := CheckConfigMapSize(big)
	if !warn.Warn || warn.Block {
		t.Fatalf("expected warning only, got %+v", warn)
	}

	huge := map[string]string{"a": string(make([]byte, ConfigMapSizeLimitBytes+1))}
	block := CheckConfigMapSize(huge)
	if !block.Block {
		t.Fatalf("expected block, got %+v", block)
	}
}
