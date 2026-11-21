package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"citadel-seeder/internal/platform"
)

func TestPotatoSettingValues_GPUVariants(t *testing.T) {
	// 1. NVIDIA RTX
	nvidiaRTX := platform.GPUInfo{
		VendorID: platform.VendorIDNVIDIA,
		Vendor:   platform.GPUVendorNVIDIA,
		Model:    "NVIDIA GeForce RTX 4070",
		IsRTX:    true,
	}
	valuesRTX := potatoSettingValues(15, true, nvidiaRTX)
	userSec := valuesRTX[squadUserSettingsSection]

	if userSec["upscalemode"] != "2" {
		t.Errorf("RTX UpscaleMode = %q, want 2 (DLSS)", userSec["upscalemode"])
	}
	if userSec["dlssmode"] != "UltraPerformance" {
		t.Errorf("RTX DLSSMode = %q, want UltraPerformance", userSec["dlssmode"])
	}

	// RAM optimization keys in user section
	expectedRAMKeys := map[string]string{
		"buncaptexturepoolsize":        "False",
		"texturestreampoolsizestorage": "(Value=0)",
		"materialquality":              "(Value=0)",
		"oceanquality":                 "(Value=0)",
		"wakesim":                      "(Value=0)",
		"skeletalmeshlodbias":          "(Value=1)",
		"foliageminlod":                "(Value=1)",
		"scoperesolutionscale":         "(Value=0)",
		"scopeupdaterate":              "(Value=30)",
	}
	for k, v := range expectedRAMKeys {
		if userSec[k] != v {
			t.Errorf("userSec[%q] = %q, want %q", k, userSec[k], v)
		}
	}

	// Scalability groups
	sg := valuesRTX["scalabilitygroups"]
	expectedSG := map[string]string{
		"sg.landscapequality":          "0",
		"sg.globalilluminationquality": "0",
		"sg.reflectionquality":         "0",
	}
	for k, v := range expectedSG {
		if sg[k] != v {
			t.Errorf("sg[%q] = %q, want %q", k, sg[k], v)
		}
	}

	// 2. AMD Radeon
	amdGPU := platform.GPUInfo{
		VendorID: platform.VendorIDAMD,
		Vendor:   platform.GPUVendorAMD,
		Model:    "AMD Radeon RX 7800 XT",
		IsRTX:    false,
	}
	valuesAMD := potatoSettingValues(15, false, amdGPU)
	userAMD := valuesAMD[squadUserSettingsSection]
	if userAMD["upscalemode"] != "1" {
		t.Errorf("AMD UpscaleMode = %q, want 1 (FSR)", userAMD["upscalemode"])
	}
	if userAMD["fsrquality"] != "(Value=4)" {
		t.Errorf("AMD FSRQuality = %q, want (Value=4)", userAMD["fsrquality"])
	}
	if _, exists := userAMD["selectedframegeneration"]; exists {
		t.Errorf("AMD should not have selectedframegeneration forced during potato mode")
	}

	// 3. Intel / NVIDIA GTX
	intelGPU := platform.GPUInfo{
		VendorID: platform.VendorIDIntel,
		Vendor:   platform.GPUVendorIntel,
		Model:    "Intel UHD Graphics 630",
		IsRTX:    false,
	}
	valuesIntel := potatoSettingValues(15, false, intelGPU)
	userIntel := valuesIntel[squadUserSettingsSection]
	if userIntel["upscalemode"] != "1" {
		t.Errorf("Intel UpscaleMode = %q, want 1 (FSR)", userIntel["upscalemode"])
	}
	if userIntel["fsrquality"] != "(Value=4)" {
		t.Errorf("Intel FSRQuality = %q, want (Value=4)", userIntel["fsrquality"])
	}
	if _, exists := userIntel["selectedframegeneration"]; exists {
		t.Errorf("Intel should not have selectedframegeneration")
	}
}

func TestPatchEngineIni_InjectsSystemSettings(t *testing.T) {
	// Original without SystemSettings or t.MaxFPS
	original := []byte("[Core.System]\r\nPaths=../../../Squad/Content\r\n")
	patched := string(patchEngineIni(original, 15))

	expectedSettings := []string{
		"[SystemSettings]",
		"r.Streaming.PoolSize=150",
		"r.Streaming.LimitPoolSizeToVRAM=1",
		"r.Streaming.UseFixedPoolSize=1",
		"r.Streaming.MaxTempMemoryAllowed=20",
		"r.Shaders.Optimize=1",
		"r.CreateShadersOnLoad=0",
	}

	for _, setting := range expectedSettings {
		if !strings.Contains(patched, setting) {
			t.Errorf("patchEngineIni missing %q, got:\n%s", setting, patched)
		}
	}
}

func TestPatchPotatoIni_UpdatesAllRAMAndUpscalingKeysInRealisticConfig(t *testing.T) {
	realisticConfig := []byte(`[/Script/Squad.SQGameUserSettings]
FrameRateLimit=144.000000
bUseVSync=True
bUncapTexturePoolSize=True
TextureStreamPoolSizeStorage=(Value=1000)
MaterialQuality=(Value=3)
OceanQuality=(Value=3)
WakeSim=(Value=3)
SkeletalMeshLODBias=(Value=0)
FoliageMinLOD=(Value=0)
ScopeResolutionScale=(Value=100)
ScopeUpdateRate=(Value=60)
UpscaleMode=0
DLSSMode=Quality
FSRQuality=(Value=2)
SelectedFrameGeneration=None

[ScalabilityGroups]
sg.ResolutionQuality=100.000000
sg.ViewDistanceQuality=3
sg.AntiAliasingQuality=3
sg.ShadowQuality=3
sg.PostProcessQuality=3
sg.TextureQuality=3
sg.EffectsQuality=3
sg.FoliageQuality=3
sg.ShadingQuality=3
sg.LandscapeQuality=3
sg.GlobalIlluminationQuality=3
sg.ReflectionQuality=3
`)

	// 1. Test with NVIDIA RTX
	rtxGPU := platform.GPUInfo{
		VendorID: platform.VendorIDNVIDIA,
		Vendor:   platform.GPUVendorNVIDIA,
		Model:    "NVIDIA GeForce RTX 4080",
		IsRTX:    true,
	}
	patchedRTX := string(patchPotatoIni(realisticConfig, 15, true, rtxGPU))

	expectedKeysRTX := []string{
		"FrameRateLimit=15.000000",
		"bUncapTexturePoolSize=False",
		"TextureStreamPoolSizeStorage=(Value=0)",
		"MaterialQuality=(Value=0)",
		"OceanQuality=(Value=0)",
		"WakeSim=(Value=0)",
		"SkeletalMeshLODBias=(Value=1)",
		"FoliageMinLOD=(Value=1)",
		"ScopeResolutionScale=(Value=0)",
		"ScopeUpdateRate=(Value=30)",
		"UpscaleMode=2",
		"DLSSMode=UltraPerformance",
		"ResolutionSizeX=640",
		"ResolutionSizeY=480",
		"sg.LandscapeQuality=0",
		"sg.GlobalIlluminationQuality=0",
		"sg.ReflectionQuality=0",
		"sg.ResolutionQuality=25.000000",
	}
	for _, expected := range expectedKeysRTX {
		if !strings.Contains(patchedRTX, expected) {
			t.Errorf("patchPotatoIni (RTX) missing expected key %q, got:\n%s", expected, patchedRTX)
		}
	}

	// 2. Test with AMD Radeon (should configure FSR + AMDFG)
	amdGPU := platform.GPUInfo{
		VendorID: platform.VendorIDAMD,
		Vendor:   platform.GPUVendorAMD,
		Model:    "AMD Radeon RX 7900 XTX",
		IsRTX:    false,
	}
	patchedAMD := string(patchPotatoIni(realisticConfig, 15, false, amdGPU))

	expectedKeysAMD := []string{
		"bUncapTexturePoolSize=False",
		"TextureStreamPoolSizeStorage=(Value=0)",
		"UpscaleMode=1",
		"FSRQuality=(Value=4)",
		"SelectedFrameGeneration=None",
		"sg.LandscapeQuality=0",
		"sg.GlobalIlluminationQuality=0",
		"sg.ReflectionQuality=0",
	}
	for _, expected := range expectedKeysAMD {
		if !strings.Contains(patchedAMD, expected) {
			t.Errorf("patchPotatoIni (AMD) missing expected key %q, got:\n%s", expected, patchedAMD)
		}
	}

	// 3. Test clean revert with mergePotatoIni
	reverted, err := mergePotatoIni(realisticConfig, []byte(patchedRTX), []byte(patchedRTX))
	if err != nil {
		t.Fatalf("mergePotatoIni failed: %v", err)
	}
	revertedStr := string(reverted)
	if !strings.Contains(revertedStr, "FrameRateLimit=144.000000") {
		t.Errorf("reverted config missing original FrameRateLimit: %s", revertedStr)
	}
	if !strings.Contains(revertedStr, "bUncapTexturePoolSize=True") {
		t.Errorf("reverted config should restore bUncapTexturePoolSize=True: %s", revertedStr)
	}
	if !strings.Contains(revertedStr, "DLSSMode=Quality") {
		t.Errorf("reverted config should restore DLSSMode=Quality: %s", revertedStr)
	}
}

func TestValidatePotatoSettings_ShowEmptyServers(t *testing.T) {
	config := []byte(`
[/Script/Squad.SQGameUserSettings]
bShowEmptyServers=True
FrameRateLimit=144.000000
bUseVSync=False
bUncapTexturePoolSize=False
`)
	if err := validatePotatoSettings(config, 15, false); err != nil {
		t.Fatalf("validatePotatoSettings rejected bShowEmptyServers=True: %v", err)
	}

	configFalse := []byte(`
[/Script/Squad.SQGameUserSettings]
bShowEmptyServers=False
FrameRateLimit=144.000000
`)
	if err := validatePotatoSettings(configFalse, 15, false); err != nil {
		t.Fatalf("validatePotatoSettings rejected bShowEmptyServers=False: %v", err)
	}

	configInvalid := []byte(`
[/Script/Squad.SQGameUserSettings]
bShowEmptyServers=NotABool
FrameRateLimit=144.000000
`)
	if err := validatePotatoSettings(configInvalid, 15, false); err == nil {
		t.Fatal("validatePotatoSettings should reject invalid boolean for bShowEmptyServers")
	}
}

func TestValidatePotatoSettings_UserDiskConfig(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("user home not found")
	}
	userConfigPath := filepath.Join(home, ".local/share/Steam/steamapps/compatdata/393380/pfx/drive_c/users/steamuser/AppData/Local/SquadGame/Saved/Config/Windows/GameUserSettings.ini")
	data, err := os.ReadFile(userConfigPath)
	if err != nil {
		t.Skip("user config not found")
	}
	if err := validatePotatoSettings(data, 15, false); err != nil {
		t.Fatalf("validatePotatoSettings failed on user disk config: %v", err)
	}
}

