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

	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	stringBuilder := strings.Builder{}

	for _, key := range keys {
		stringBuilder.WriteString(key)
		stringBuilder.WriteRune('\u0000')
		stringBuilder.WriteString(data[key])
		stringBuilder.WriteRune('\n')
	}

	hashSum := sha256.Sum256([]byte(stringBuilder.String()))

	return hex.EncodeToString(hashSum[:])
}
