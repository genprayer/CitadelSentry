//go:build !windows

package platform

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func detectGPULinux() GPUInfo {
	// Look for card devices in /sys/class/drm/card*
	matches, _ := filepath.Glob("/sys/class/drm/card[0-9]")
	for _, card := range matches {
		vendorBytes, err := os.ReadFile(filepath.Join(card, "device/vendor"))
		if err != nil {
			continue
		}
		vendorStr := strings.TrimSpace(string(vendorBytes))
		vID64, _ := strconv.ParseUint(strings.TrimPrefix(vendorStr, "0x"), 16, 32)
		vendorID := uint32(vID64)

		model := "Linux Display Adapter"
		vendor, isRTX := ParseGPUVendor(vendorID, "")
		switch vendor {
		case GPUVendorNVIDIA:
			model = "NVIDIA Graphics Device"
		case GPUVendorAMD:
			model = "AMD Radeon Graphics"
		case GPUVendorIntel:
			model = "Intel HD/UHD Graphics"
		}

		if vendor != GPUVendorUnknown {
			return GPUInfo{
				VendorID: vendorID,
				Vendor:   vendor,
				Model:    model,
				IsRTX:    isRTX,
			}
		}
	}

	return GPUInfo{
		Vendor: GPUVendorUnknown,
		Model:  "Linux Graphics Adapter (Proton/Vulkan)",
	}
}

func DetectGPU() GPUInfo {
	return detectGPULinux()
}
