package keymap

import (
	"bytes"
	"testing"

	"github.com/bluehandle-open/bluehandle-go/internal/consts"
)

func TestTablesMatchJava(t *testing.T) {
	// Literal values copied from KeySetting.java.
	cases := []struct {
		typ  byte
		want []byte
	}{
		{consts.TypeHandleKeySet, []byte{87, 83, 65, 68, 85, 73, 79, 74, 75, 76, 49, 53}},
		{consts.TypePptKeySet, []byte{38, 40, 37, 39, 27, 16, 116}},
		{consts.TypePlayerKeySet, []byte{17, 18, 116, 17, 18, 117, 17, 18, 37, 17, 18, 39, 17, 18, 38, 17, 18, 40}},
	}
	for _, c := range cases {
		got, ok := ByType(c.typ)
		if !ok || !bytes.Equal(got, c.want) {
			t.Errorf("type 0x%02x: got %v, want %v", c.typ, got, c.want)
		}
	}
	if _, ok := ByType(consts.TypeKey); ok {
		t.Error("TypeKey must not resolve to a table")
	}
}

func TestByTypeReturnsCopy(t *testing.T) {
	got, _ := ByType(consts.TypeHandleKeySet)
	got[0] = 0
	if Handle[0] == 0 {
		t.Fatal("ByType leaked the shared slice")
	}
}

func TestKeyName(t *testing.T) {
	if KeyName('W') != "W" || KeyName(VKF5) != "F5" || KeyName(200) != "vk200" {
		t.Fatal("unexpected key names")
	}
}
