package telemetry

import (
	"time"
)

// Checkpoint represents the lifecycle moment when telemetry is sampled.
type Checkpoint string

const (
	CheckpointT0         Checkpoint = "t0_pre_launch"
	CheckpointPostLaunch Checkpoint = "t1_post_launch"
	Checkpoint5m         Checkpoint = "t5_5m"
	Checkpoint15m        Checkpoint = "t15_15m"
	Checkpoint30m        Checkpoint = "t30_30m"
	Checkpoint60m        Checkpoint = "t60_60m"
	CheckpointExit       Checkpoint = "t_exit"
)

// HardwareConfig describes the static, anonymous hardware and OS baseline.
type HardwareConfig struct {
	OSCaption        string `json:"osCaption"`        // e.g. "Microsoft Windows 11 Pro"
	OSBuild          string `json:"osBuild"`          // e.g. "22631.3880"
	OSArchitecture   string `json:"osArchitecture"`   // e.g. "64-bit"
	IsElevated       bool   `json:"isElevated"`       // true if launched as Admin
	CPUModel         string `json:"cpuModel"`         // e.g. "AMD Ryzen 7 7800X3D 8-Core Processor"
	CPUCores         int    `json:"cpuCores"`         // Physical cores
	CPUThreads       int    `json:"cpuThreads"`       // Logical threads
	RAMTotalMB       uint64 `json:"ramTotalMb"`       // Total physical memory in MB
	GPUModel         string `json:"gpuModel"`         // e.g. "NVIDIA GeForce RTX 4080"
	GPUVendor        string `json:"gpuVendor"`        // "NVIDIA", "AMD", "Intel"
	GPUVRAMMB        uint64 `json:"gpuVramMb"`        // Dedicated VRAM in MB
	GPUDriverVersion string `json:"gpuDriverVersion"` // Driver version string
	DisplayRes       string `json:"displayResolution"` // e.g. "2560x1440@144Hz"
	ActivePowerPlan  string `json:"activePowerPlan"`  // e.g. "Balanced", "High Performance"
	DriveType        string `json:"driveType"`        // "NVMe SSD", "SATA SSD", "HDD", "Unknown"
	SquadVersion     string `json:"squadVersion"`     // Detected Squad build version
	SentryVersion    string `json:"sentryVersion"`    // Citadel Sentry build version
}

// DynamicMetrics captures runtime consumption at a specific checkpoint.
type DynamicMetrics struct {
	Timestamp            time.Time  `json:"timestamp"`
	Checkpoint           Checkpoint `json:"checkpoint"`
	SystemCPUPercent     float64    `json:"systemCpuPercent"`
	SquadCPUPercent      float64    `json:"squadCpuPercent"`
	SquadWorkingSetMB    uint64     `json:"squadWorkingSetMb"`
	SquadPrivateBytesMB  uint64     `json:"squadPrivateBytesMb"`
	SystemAvailableRAMMB uint64     `json:"systemAvailableRamMb"`
	SquadVRAMUsedMB      uint64     `json:"squadVramUsedMb,omitempty"`
	SquadFPS             float64    `json:"squadFps,omitempty"`
	SquadPageFaultCount  uint64     `json:"squadPageFaultCount"`
	SquadThreadCount     int        `json:"squadThreadCount"`
	SquadDiskIOReadMB    float64    `json:"squadDiskIoReadMb"`
	SquadDiskIOWriteMB   float64    `json:"squadDiskIoWriteMb"`
}

// SeedingDiagnostics logs the status of potato mode and runtime adjustments.
type SeedingDiagnostics struct {
	PotatoModeEnabled        bool   `json:"potatoModeEnabled"`
	FpsLimitTarget           int    `json:"fpsLimitTarget"`
	LowResolutionActive      bool   `json:"lowResolutionActive"`
	CpuPriorityStatus        string `json:"cpuPriorityStatus"` // "Applied", "AccessDenied_EAC", "NotSupported"
	EcoQoSStatus             string `json:"ecoQosStatus"`      // "Applied", "AccessDenied_EAC", "NotSupported"
	VramOptimizationApplied  bool   `json:"vramOptimizationApplied"`
	AudioThrottlingApplied   bool   `json:"audioThrottlingApplied"`
	NvidiaProfileApplied     bool   `json:"nvidiaProfileApplied,omitempty"`
	SessionDurationSeconds   int64  `json:"sessionDurationSeconds"`
	ExitReason               string `json:"exitReason"` // "TargetReached", "UserStop", "Timeout", "GameCrash", "Error"
	OptimizationErrors       []string `json:"optimizationErrors,omitempty"`
}

// Payload represents a complete, anonymized diagnostic report.
type Payload struct {
	SchemaVersion string             `json:"schemaVersion"` // "1.0.0"
	SessionUUID   string             `json:"sessionUuid"`   // Ephemeral UUID generated on startup
	Hardware      HardwareConfig     `json:"hardware"`
	Diagnostics   SeedingDiagnostics `json:"diagnostics"`
	Samples       []DynamicMetrics   `json:"samples"`
}
