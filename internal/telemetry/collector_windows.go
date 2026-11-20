//go:build windows

package telemetry

import (
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modpsapi               = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = modpsapi.NewProc("GetProcessMemoryInfo")
	modkernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
)

type processMemoryCountersEx struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
	privateUsage               uintptr
}

type memorystatusex struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// CollectDynamicSample samples current system and Squad memory/CPU statistics on Windows.
func CollectDynamicSample(checkpoint Checkpoint) DynamicMetrics {
	sample := DynamicMetrics{
		Timestamp:  time.Now(),
		Checkpoint: checkpoint,
	}

	// 1. System Memory via GlobalMemoryStatusEx
	var memStatus memorystatusex
	memStatus.cbSize = uint32(unsafe.Sizeof(memStatus))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret != 0 {
		sample.SystemAvailableRAMMB = memStatus.ullAvailPhys / (1024 * 1024)
	}

	// 2. Locate Squad process
	pid := findSquadPID()
	if pid == 0 {
		return sample
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return sample
	}
	defer windows.CloseHandle(handle)

	// 3. Process Memory Counters via GetProcessMemoryInfo
	var counters processMemoryCountersEx
	counters.cb = uint32(unsafe.Sizeof(counters))
	ret, _, _ = procGetProcessMemoryInfo.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.cb),
	)
	if ret != 0 {
		sample.SquadWorkingSetMB = uint64(counters.workingSetSize / (1024 * 1024))
		sample.SquadPrivateBytesMB = uint64(counters.pagefileUsage / (1024 * 1024))
		sample.SquadPageFaultCount = uint64(counters.pageFaultCount)
	}

	return sample
}

func findSquadPID() uint32 {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return 0
	}
	for {
		name := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		base := strings.ToLower(filepath.Base(name))
		if base == "squadgame.exe" || base == "squadgame-win64-shipping.exe" {
			return entry.ProcessID
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return 0
}
