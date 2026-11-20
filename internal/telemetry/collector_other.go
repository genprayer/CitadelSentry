//go:build !windows

package telemetry

import (
	"time"
)

// CollectDynamicSample provides a non-Windows fallback stub for development and testing.
func CollectDynamicSample(checkpoint Checkpoint) DynamicMetrics {
	return DynamicMetrics{
		Timestamp:            time.Now(),
		Checkpoint:           checkpoint,
		SystemAvailableRAMMB: 8192,
		SquadWorkingSetMB:    6144,
		SquadPrivateBytesMB:  6500,
		SquadPageFaultCount:  1024,
	}
}
