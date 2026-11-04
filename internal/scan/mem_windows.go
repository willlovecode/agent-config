package scan

import (
	"math"
	"syscall"
	"unsafe"
)

type memoryStatusEx struct {
	Length, MemoryLoad                  uint32
	TotalPhys, AvailPhys, TotalPageFile uint64
	AvailPageFile, TotalVirtual         uint64
	AvailVirtual, AvailExtendedVirtual  uint64
}

func ramGB() float64 {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	if ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&m))); ok == 0 {
		return 0
	}
	return math.Round(float64(m.TotalPhys) / (1 << 30))
}
