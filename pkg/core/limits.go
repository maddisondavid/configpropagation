package core

const (
	// ConfigMapSizeLimitBytes approximates the maximum payload for a ConfigMap.
	ConfigMapSizeLimitBytes = 1048576 // 1MiB
	// ConfigMapSizeWarnThresholdBytes raises a warning when above ~90%% of the limit.
	ConfigMapSizeWarnThresholdBytes = ConfigMapSizeLimitBytes * 9 / 10
)

// SizeCheckResult captures the outcome of validating a ConfigMap payload size.
type SizeCheckResult struct {
	Bytes int
	Warn  bool
	Block bool
}

// CheckConfigMapSize computes the serialized size of data to guard against large payloads.
func CheckConfigMapSize(data map[string]string) SizeCheckResult {
	total := 0
	for k, v := range data {
		total += len(k) + len(v)
	}
	res := SizeCheckResult{Bytes: total}
	if total > ConfigMapSizeLimitBytes {
		res.Block = true
	} else if total > ConfigMapSizeWarnThresholdBytes {
		res.Warn = true
	}
	return res
}
