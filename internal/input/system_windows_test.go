package input

import (
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/bluehandle-open/bluehandle-go/internal/keymap"
)

func TestInputStructSizesMatchWin32(t *testing.T) {
	// sizeof(INPUT) is 40 on 64-bit Windows and 28 on 32-bit.
	want := uintptr(28)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want = 40
	}
	if got := unsafe.Sizeof(keyInput{}); got != want {
		t.Errorf("keyInput is %d bytes, want %d", got, want)
	}
	if got := unsafe.Sizeof(mouseButtonInput{}); got != want {
		t.Errorf("mouseButtonInput is %d bytes, want %d", got, want)
	}
}

func TestKeyInputFlags(t *testing.T) {
	if f := keyInputFor('W', 0x11, false).ki.flags; f != 0 {
		t.Errorf("W down flags %#x", f)
	}
	if f := keyInputFor(keymap.VKUp, 0, true).ki.flags; f != keyeventfExtendedKey|keyeventfKeyUp {
		t.Errorf("Up release flags %#x", f)
	}
	if f := keyInputFor(keymap.VKMediaPlayPause, 0, false).ki.flags; f != keyeventfExtendedKey {
		t.Errorf("play/pause flags %#x", f)
	}
	if in := keyInputFor('W', 0x11, false); in.typ != inputKeyboard || in.ki.vk != 'W' || in.ki.scan != 0x11 {
		t.Errorf("got %+v", in)
	}
}

func TestMouseButtons(t *testing.T) {
	m, ok := mouseInputFor(vkRButton, true)
	if !ok || m.typ != inputMouse || m.mi.flags != mouseeventfRightUp {
		t.Fatalf("right up: %+v %v", m, ok)
	}
	if _, ok := mouseInputFor('A', false); ok {
		t.Fatal("A is not a mouse button")
	}
}

func TestTextInputs(t *testing.T) {
	type ev struct {
		vk, scan uint16
		flags    uint32
	}
	var got []ev
	for _, in := range textInputs("a你\r\n😀") {
		got = append(got, ev{in.ki.vk, in.ki.scan, in.ki.flags})
	}
	u, up := uint32(keyeventfUnicode), uint32(keyeventfUnicode|keyeventfKeyUp)
	want := []ev{
		{0, 'a', u}, {0, 'a', up},
		{0, 0x4F60, u}, {0, 0x4F60, up}, // 你
		{keymap.VKEnter, 0, 0}, {keymap.VKEnter, 0, keyeventfKeyUp}, // CRLF is one Enter
		{0, 0xD83D, u}, {0, 0xD83D, up}, // 😀 as a surrogate pair
		{0, 0xDE00, u}, {0, 0xDE00, up},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if textInputs("") != nil {
		t.Fatal("empty text must produce no input")
	}
}

// TestSendInputLive injects F24, which no common program binds, to check
// that Windows accepts the INPUT layout. It touches the real desktop, so it
// only runs when BLUEHANDLE_LIVE_INPUT=1.
func TestSendInputLive(t *testing.T) {
	if os.Getenv("BLUEHANDLE_LIVE_INPUT") != "1" {
		t.Skip("set BLUEHANDLE_LIVE_INPUT=1 to inject a real F24 key press")
	}
	kb, err := NewSystem()
	if err != nil {
		t.Fatal(err)
	}
	const vkF24 = 0x87
	if err := kb.Press(vkF24); err != nil {
		t.Fatal(err)
	}
	if err := kb.(Releaser).ReleaseAll(); err != nil {
		t.Fatal(err)
	}
	if err := kb.TypeText(""); err != nil {
		t.Fatal(err)
	}
}
