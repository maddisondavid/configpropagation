package core

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// HashData computes a stable sha256 hash of the string data map.
// Keys are sorted and joined as key\u0000value pairs to avoid JSON map nondeterminism.
func HashData(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b := strings.Builder{}
	for _, k := range keys {
		b.WriteString(k)
		b.WriteRune('\u0000')
		b.WriteString(data[k])
		b.WriteRune('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
