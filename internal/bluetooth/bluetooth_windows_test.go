package bluetooth

import (
	"testing"
	"unsafe"
)

// The Win32 functions check dwSize, so the Go mirrors must match the C sizes.
func TestStructSizes(t *testing.T) {
	want := map[string][2]uintptr{ // name: {386, amd64/arm64}
		"BLUETOOTH_DEVICE_INFO":          {560, 560},
		"BLUETOOTH_DEVICE_SEARCH_PARAMS": {32, 40},
	}
	got := map[string]uintptr{
		"BLUETOOTH_DEVICE_INFO":          unsafe.Sizeof(deviceInfo{}),
		"BLUETOOTH_DEVICE_SEARCH_PARAMS": unsafe.Sizeof(searchParams{}),
	}
	i := 1
	if unsafe.Sizeof(uintptr(0)) == 4 {
		i = 0
	}
	for name, sizes := range want {
		if got[name] != sizes[i] {
			t.Errorf("%s: size %d, want %d", name, got[name], sizes[i])
		}
	}
	if off := unsafe.Offsetof(deviceInfo{}.address); off != 8 {
		t.Errorf("BLUETOOTH_DEVICE_INFO.Address at %d, want 8", off)
	}
	if off := unsafe.Offsetof(deviceInfo{}.name); off != 64 {
		t.Errorf("BLUETOOTH_DEVICE_INFO.szName at %d, want 64", off)
	}
}
