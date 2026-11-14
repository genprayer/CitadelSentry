//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type dxgiAdapterDesc struct {
	Description           [128]uint16
	VendorID              uint32
	DeviceID              uint32
	SubSysID              uint32
	Revision              uint32
	DedicatedVideoMemory  uintptr
	DedicatedSystemMemory uintptr
	SharedSystemMemory    uintptr
	AdapterLuid           int64
}

var (
	modDXGI               = syscall.NewLazyDLL("dxgi.dll")
	procCreateDXGIFactory = modDXGI.NewProc("CreateDXGIFactory")
	guidIDXGIFactory      = windows.GUID{Data1: 0x7b7166ec, Data2: 0x21c7, Data3: 0x44ae, Data4: [8]byte{0xb2, 0x1a, 0xc9, 0xae, 0x32, 0x1a, 0xe3, 0x69}}
)

func detectGPUWindows() GPUInfo {
	if info, err := detectGPUDXGI(); err == nil && info.Vendor != GPUVendorUnknown {
		return info
	}
	if info, err := detectGPURegistry(); err == nil && info.Vendor != GPUVendorUnknown {
		return info
	}
	if info, err := detectGPUWMI(); err == nil && info.Vendor != GPUVendorUnknown {
		return info
	}
	return GPUInfo{
		Vendor: GPUVendorUnknown,
		Model:  "Standard Graphics Adapter",
	}
}

func detectGPUDXGI() (GPUInfo, error) {
	if err := modDXGI.Load(); err != nil {
		return GPUInfo{}, err
	}
	var factory uintptr
	hr, _, _ := procCreateDXGIFactory.Call(
		uintptr(unsafe.Pointer(&guidIDXGIFactory)),
		uintptr(unsafe.Pointer(&factory)),
	)
	if hr != 0 || factory == 0 {
		return GPUInfo{}, fmt.Errorf("CreateDXGIFactory failed: 0x%08X", hr)
	}

	factoryVtbl := *(*uintptr)(unsafe.Pointer(factory))
	enumAdaptersFn := *(*uintptr)(unsafe.Pointer(factoryVtbl + 7*unsafe.Sizeof(uintptr(0))))
	factoryReleaseFn := *(*uintptr)(unsafe.Pointer(factoryVtbl + 2*unsafe.Sizeof(uintptr(0))))
	defer syscall.SyscallN(factoryReleaseFn, factory)

	var bestGPU GPUInfo
	var maxVRAM uint64

	for i := uint32(0); ; i++ {
		var adapter uintptr
		hr, _, _ := syscall.SyscallN(enumAdaptersFn, factory, uintptr(i), uintptr(unsafe.Pointer(&adapter)))
		if hr != 0 || adapter == 0 {
			break
		}
		adapterVtbl := *(*uintptr)(unsafe.Pointer(adapter))
		getDescFn := *(*uintptr)(unsafe.Pointer(adapterVtbl + 8*unsafe.Sizeof(uintptr(0))))
		adapterReleaseFn := *(*uintptr)(unsafe.Pointer(adapterVtbl + 2*unsafe.Sizeof(uintptr(0))))

		var desc dxgiAdapterDesc
		hr, _, _ = syscall.SyscallN(getDescFn, adapter, uintptr(unsafe.Pointer(&desc)))
		syscall.SyscallN(adapterReleaseFn, adapter)

		if hr != 0 {
			continue
		}

		model := windows.UTF16ToString(desc.Description[:])
		if desc.VendorID == 0x1414 || strings.Contains(strings.ToLower(model), "basic render") {
			continue
		}
		vram := uint64(desc.DedicatedVideoMemory)
		vendor, isRTX := ParseGPUVendor(desc.VendorID, model)

		// Prefer discrete GPUs (NVIDIA/AMD) over integrated (Intel/Microsoft Basic Display)
		isDiscrete := vendor == GPUVendorNVIDIA || vendor == GPUVendorAMD
		bestIsDiscrete := bestGPU.Vendor == GPUVendorNVIDIA || bestGPU.Vendor == GPUVendorAMD

		if bestGPU.Vendor == GPUVendorUnknown || (isDiscrete && !bestIsDiscrete) || (isDiscrete == bestIsDiscrete && vram > maxVRAM) {
			maxVRAM = vram
			bestGPU = GPUInfo{
				VendorID:      desc.VendorID,
				Vendor:        vendor,
				Model:         model,
				DedicatedVRAM: vram,
				IsRTX:         isRTX,
			}
		}
	}

	if bestGPU.Model != "" {
		return bestGPU, nil
	}
	return GPUInfo{}, fmt.Errorf("no DXGI adapters found")
}

func detectGPURegistry() (GPUInfo, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`, registry.READ)
	if err != nil {
		return GPUInfo{}, err
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return GPUInfo{}, err
	}

	var bestGPU GPUInfo
	for _, sub := range subkeys {
		if !strings.HasPrefix(sub, "0") {
			continue
		}
		sk, err := registry.OpenKey(k, sub, registry.READ)
		if err != nil {
			continue
		}
		desc, _, err := sk.GetStringValue("DriverDesc")
		matchID, _, _ := sk.GetStringValue("MatchingDeviceId")
		memSize, _, _ := sk.GetIntegerValue("HardwareInformation.qwMemorySize")
		if memSize == 0 {
			if dw, _, err := sk.GetIntegerValue("HardwareInformation.MemorySize"); err == nil {
				memSize = dw
			}
		}
		sk.Close()

		if err != nil || desc == "" {
			continue
		}

		var vendorID uint32
		upperMatch := strings.ToUpper(matchID)
		if strings.Contains(upperMatch, "VEN_10DE") {
			vendorID = VendorIDNVIDIA
		} else if strings.Contains(upperMatch, "VEN_1002") {
			vendorID = VendorIDAMD
		} else if strings.Contains(upperMatch, "VEN_8086") {
			vendorID = VendorIDIntel
		}

		vendor, isRTX := ParseGPUVendor(vendorID, desc)
		if vendor != GPUVendorUnknown {
			isDiscrete := vendor == GPUVendorNVIDIA || vendor == GPUVendorAMD
			bestIsDiscrete := bestGPU.Vendor == GPUVendorNVIDIA || bestGPU.Vendor == GPUVendorAMD
			if bestGPU.Vendor == GPUVendorUnknown || (isDiscrete && !bestIsDiscrete) || (isDiscrete == bestIsDiscrete && memSize > bestGPU.DedicatedVRAM) {
				bestGPU = GPUInfo{
					VendorID:      vendorID,
					Vendor:        vendor,
					Model:         desc,
					DedicatedVRAM: memSize,
					IsRTX:         isRTX,
				}
			}
		}
	}
	if bestGPU.Vendor != GPUVendorUnknown {
		return bestGPU, nil
	}
	return GPUInfo{}, fmt.Errorf("no GPU found in registry")
}

func detectGPUWMI() (GPUInfo, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Get-CimInstance Win32_VideoController | Select-Object -Property Name, PNPDeviceID, AdapterRAM | ConvertTo-Csv -NoTypeInformation")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	out, err := cmd.Output()
	if err != nil {
		return GPUInfo{}, err
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return GPUInfo{}, fmt.Errorf("no WMI video output")
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(trimmed, ",")
		if len(parts) >= 2 {
			name := strings.Trim(parts[0], `"`)
			pnp := strings.ToUpper(strings.Trim(parts[1], `"`))
			var vendorID uint32
			if strings.Contains(pnp, "VEN_10DE") {
				vendorID = VendorIDNVIDIA
			} else if strings.Contains(pnp, "VEN_1002") {
				vendorID = VendorIDAMD
			} else if strings.Contains(pnp, "VEN_8086") {
				vendorID = VendorIDIntel
			}
			vendor, isRTX := ParseGPUVendor(vendorID, name)
			if vendor != GPUVendorUnknown {
				return GPUInfo{
					VendorID: vendorID,
					Vendor:   vendor,
					Model:    name,
					IsRTX:    isRTX,
				}, nil
			}
		}
	}
	return GPUInfo{}, fmt.Errorf("WMI GPU parsing failed")
}

func DetectGPU() GPUInfo {
	return detectGPUWindows()
}
