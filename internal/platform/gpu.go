package platform

import (
	"strings"
)

type GPUVendor string

const (
	GPUVendorUnknown GPUVendor = "unknown"
	GPUVendorNVIDIA  GPUVendor = "nvidia"
	GPUVendorAMD     GPUVendor = "amd"
	GPUVendorIntel   GPUVendor = "intel"
)

const (
	VendorIDNVIDIA uint32 = 0x10DE
	VendorIDAMD    uint32 = 0x1002
	VendorIDIntel  uint32 = 0x8086
)

type GPUInfo struct {
	VendorID      uint32    `json:"vendorId"`
	Vendor        GPUVendor `json:"vendor"`
	Model         string    `json:"model"`
	DedicatedVRAM uint64    `json:"dedicatedVram"` // in bytes
	IsRTX         bool      `json:"isRtx"`
}

// ParseGPUVendor identifies the vendor and RTX capability from vendor ID and model name.
func ParseGPUVendor(vendorID uint32, model string) (GPUVendor, bool) {
	upperModel := strings.ToUpper(model)
	var vendor GPUVendor
	switch vendorID {
	case VendorIDNVIDIA:
		vendor = GPUVendorNVIDIA
	case VendorIDAMD:
		vendor = GPUVendorAMD
	case VendorIDIntel:
		vendor = GPUVendorIntel
	default:
		if strings.Contains(upperModel, "NVIDIA") || strings.Contains(upperModel, "GEFORCE") {
			vendor = GPUVendorNVIDIA
		} else if strings.Contains(upperModel, "AMD") || strings.Contains(upperModel, "RADEON") {
			vendor = GPUVendorAMD
		} else if strings.Contains(upperModel, "INTEL") {
			vendor = GPUVendorIntel
		} else {
			vendor = GPUVendorUnknown
		}
	}
	isRTX := vendor == GPUVendorNVIDIA && strings.Contains(upperModel, "RTX")
	return vendor, isRTX
}
