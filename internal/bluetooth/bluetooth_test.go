package bluetooth

import "testing"

func TestParseFormatAddr(t *testing.T) {
	for _, s := range []string{"bt:AA:BB:CC:DD:EE:01", "BT:aa-bb-cc-dd-ee-01", " bt:aa:bb:cc:dd:ee:01 "} {
		v, err := ParseAddr(s)
		if err != nil {
			t.Fatalf("ParseAddr(%q): %v", s, err)
		}
		if v != 0xAABBCCDDEE01 {
			t.Fatalf("ParseAddr(%q) = %#x", s, v)
		}
		if got := FormatAddr(v); got != "bt:AA:BB:CC:DD:EE:01" {
			t.Fatalf("FormatAddr = %q", got)
		}
	}
	for _, s := range []string{"", "192.168.1.5", "bt:AA:BB:CC:DD:EE", "bt:AA:BB:CC:DD:EE:GG", "bt:AAA:BB:CC:DD:EE:F", "AA:BB:CC:DD:EE:01"} {
		if IsAddr(s) {
			t.Errorf("IsAddr(%q) = true", s)
		}
	}
}

func TestSortDevices(t *testing.T) {
	ds := []Device{
		{Name: "耳机", Connected: true},
		{Name: "b phone", Phone: true},
		{Name: "A phone", Phone: true},
		{Name: "鼠标"},
	}
	sortDevices(ds)
	want := []string{"A phone", "b phone", "耳机", "鼠标"}
	for i, d := range ds {
		if d.Name != want[i] {
			t.Fatalf("order %d = %q, want %q", i, d.Name, want[i])
		}
	}
}
