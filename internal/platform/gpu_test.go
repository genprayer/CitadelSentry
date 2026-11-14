package platform

import "testing"

func TestParseGPUVendor(t *testing.T) {
	tests := []struct {
		vendorID uint32
		model    string
		wantVend GPUVendor
		wantRTX  bool
	}{
		{VendorIDNVIDIA, "NVIDIA GeForce RTX 4070 Ti SUPER", GPUVendorNVIDIA, true},
		{VendorIDNVIDIA, "NVIDIA GeForce RTX 3080", GPUVendorNVIDIA, true},
		{VendorIDNVIDIA, "NVIDIA GeForce GTX 1080 Ti", GPUVendorNVIDIA, false},
		{VendorIDAMD, "AMD Radeon RX 7900 XTX", GPUVendorAMD, false},
		{VendorIDIntel, "Intel(R) UHD Graphics 630", GPUVendorIntel, false},
		{0, "GeForce RTX 2060", GPUVendorNVIDIA, true},
		{0, "AMD Radeon RX 6700 XT", GPUVendorAMD, false},
		{0, "Intel Iris Xe", GPUVendorIntel, false},
		{0, "Unknown Device", GPUVendorUnknown, false},
	}

	for _, tt := range tests {
		gotVend, gotRTX := ParseGPUVendor(tt.vendorID, tt.model)
		if gotVend != tt.wantVend {
			t.Errorf("ParseGPUVendor(%x, %q) vendor = %v, want %v", tt.vendorID, tt.model, gotVend, tt.wantVend)
		}
		if gotRTX != tt.wantRTX {
			t.Errorf("ParseGPUVendor(%x, %q) isRTX = %v, want %v", tt.vendorID, tt.model, gotRTX, tt.wantRTX)
		}
	}
}

func TestDetectGPU(t *testing.T) {
	gpu := DetectGPU()
	if gpu.Model == "" {
		t.Errorf("DetectGPU() returned empty model: %+v", gpu)
	}
}
