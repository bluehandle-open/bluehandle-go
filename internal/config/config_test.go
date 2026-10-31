package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, Default()) {
		t.Fatalf("got %+v", c)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	c := Default()
	c.Port = 9000
	c.ShowText = false
	at := time.Date(2026, 10, 31, 9, 37, 0, 0, time.UTC)
	c.Touch("192.168.6.144", at)
	c.SetDuration("192.168.6.144", 42*time.Minute)
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, c) {
		t.Fatalf("got %+v\nwant %+v", got, c)
	}
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".config-*")); len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestLoadKeepsDefaultsForMissingFieldsAndFixesPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"port": 0, "show_text": false}`), 0o644)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8088 || c.ShowText || !c.CloseToTray || !c.AutoReconnect {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{not json`), 0o644)
	c, err := Load(path)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !reflect.DeepEqual(c, Default()) {
		t.Fatal("a corrupt file must fall back to defaults")
	}
}

func TestRecentList(t *testing.T) {
	var c Config
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, a := range []string{"a", "b", "c", "d", "e", "f"} {
		c.Touch(a, base.Add(time.Duration(i)*time.Minute))
	}
	if got := addrs(c); !reflect.DeepEqual(got, []string{"f", "e", "d", "c", "b"}) {
		t.Fatalf("after six: %v", got)
	}
	c.SetDuration("c", 90*time.Second)
	c.Touch("c", base.Add(time.Hour))
	if got := addrs(c); !reflect.DeepEqual(got, []string{"c", "f", "e", "d", "b"}) {
		t.Fatalf("after re-touch: %v", got)
	}
	if c.Recent[0].Duration != 90 || !c.Recent[0].LastAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("re-touch lost fields: %+v", c.Recent[0])
	}
	c.Forget("f")
	c.Forget("missing")
	if got := addrs(c); !reflect.DeepEqual(got, []string{"c", "e", "d", "b"}) || c.LastAddr() != "c" {
		t.Fatalf("after forget: %v", got)
	}
}

func addrs(c Config) []string {
	var out []string
	for _, r := range c.Recent {
		out = append(out, r.Addr)
	}
	return out
}
