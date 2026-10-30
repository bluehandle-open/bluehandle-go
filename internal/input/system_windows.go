package input

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/bluehandle-open/bluehandle-go/internal/keymap"
)

// System injects input with SendInput. Wire key codes are already Windows
// virtual-key codes, so no translation table is needed; mouse button codes
// (VK_LBUTTON and friends) become mouse clicks. It needs no cgo.
//
// Windows drops input aimed at windows of a higher integrity level (UIPI),
// so keys do not reach elevated programs unless bluehandle runs elevated too.
type System struct {
	mu   sync.Mutex
	held map[int]bool // keys currently pressed, released by ReleaseAll
}

// NewSystem returns the SendInput keyboard.
func NewSystem() (Keyboard, error) {
	if err := procSendInput.Find(); err != nil {
		return nil, err
	}
	return &System{held: map[int]bool{}}, nil
}

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procSendInput       = user32.NewProc("SendInput")
	procMapVirtualKeyW  = user32.NewProc("MapVirtualKeyW")
	procLockWorkStation = user32.NewProc("LockWorkStation")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyeventfExtendedKey = 0x0001
	keyeventfKeyUp       = 0x0002
	keyeventfUnicode     = 0x0004

	mouseeventfLeftDown   = 0x0002
	mouseeventfLeftUp     = 0x0004
	mouseeventfRightDown  = 0x0008
	mouseeventfRightUp    = 0x0010
	mouseeventfMiddleDown = 0x0020
	mouseeventfMiddleUp   = 0x0040

	mapvkVKToVSC = 0

	vkLButton = 0x01
	vkRButton = 0x02
	vkMButton = 0x04
)

// keybdInput and mouseInput mirror KEYBDINPUT and MOUSEINPUT. The INPUT
// wrappers pad to the size of the C union so cbSize matches on 386 and amd64.
type keybdInput struct {
	vk        uint16
	scan      uint16
	flags     uint32
	time      uint32
	extraInfo uintptr
}

type mouseInput struct {
	dx, dy    int32
	mouseData uint32
	flags     uint32
	time      uint32
	extraInfo uintptr
}

type keyInput struct {
	typ uint32
	ki  keybdInput
	_   [8]byte
}

type mouseButtonInput struct {
	typ uint32
	mi  mouseInput
}

// extended lists keys that need KEYEVENTF_EXTENDEDKEY; without it the
// arrows and the navigation block arrive as their numeric keypad twins.
var extended = map[int]bool{
	keymap.VKPageUp: true, keymap.VKPageDown: true, 0x23: true, 0x24: true, // End, Home
	keymap.VKLeft: true, keymap.VKUp: true, keymap.VKRight: true, keymap.VKDown: true,
	0x2C: true, keymap.VKInsert: true, keymap.VKDelete: true, // PrintScreen
	keymap.VKLWin: true, 0x5C: true, keymap.VKApps: true, // RWin
	0x6F: true, 0x90: true, // numpad divide, NumLock
	0xA3: true, 0xA5: true, // right Ctrl, right Alt
	keymap.VKVolumeMute: true, keymap.VKVolumeDown: true, keymap.VKVolumeUp: true,
	keymap.VKMediaNext: true, keymap.VKMediaPrev: true, keymap.VKMediaStop: true, keymap.VKMediaPlayPause: true,
}

func (s *System) Press(vk int) error {
	if err := s.key(vk, false); err != nil {
		return err
	}
	s.mu.Lock()
	s.held[vk] = true
	s.mu.Unlock()
	return nil
}

func (s *System) Release(vk int) error {
	s.mu.Lock()
	delete(s.held, vk)
	s.mu.Unlock()
	return s.key(vk, true)
}

// ReleaseAll lifts every key still held, so a dropped connection never
// leaves a key stuck down on the PC.
func (s *System) ReleaseAll() error {
	s.mu.Lock()
	keys := make([]int, 0, len(s.held))
	for vk := range s.held {
		keys = append(keys, vk)
	}
	s.held = map[int]bool{}
	s.mu.Unlock()
	var errs []error
	for _, vk := range keys {
		errs = append(errs, s.key(vk, true))
	}
	return errors.Join(errs...)
}

func (s *System) key(vk int, up bool) error {
	if vk <= 0 || vk > 0xFE {
		return fmt.Errorf("input: invalid virtual-key code %d", vk)
	}
	if m, ok := mouseInputFor(vk, up); ok {
		return send(unsafe.Pointer(&m), 1, unsafe.Sizeof(m))
	}
	in := []keyInput{keyInputFor(vk, scanCode(vk), up)}
	return send(unsafe.Pointer(&in[0]), len(in), unsafe.Sizeof(in[0]))
}

func (s *System) TypeText(text string) error {
	in := textInputs(text)
	if len(in) == 0 {
		return nil
	}
	return send(unsafe.Pointer(&in[0]), len(in), unsafe.Sizeof(in[0]))
}

func (s *System) LockScreen() error {
	if r, _, err := procLockWorkStation.Call(); r == 0 {
		return fmt.Errorf("input: LockWorkStation: %w", err)
	}
	return nil
}

// keyInputFor builds one key event. The scan code is filled in as well,
// matching what java.awt.Robot did, because some games read it.
func keyInputFor(vk int, scan uint16, up bool) keyInput {
	var flags uint32
	if extended[vk] {
		flags |= keyeventfExtendedKey
	}
	if up {
		flags |= keyeventfKeyUp
	}
	return keyInput{typ: inputKeyboard, ki: keybdInput{vk: uint16(vk), scan: scan, flags: flags}}
}

func mouseInputFor(vk int, up bool) (mouseButtonInput, bool) {
	var down, release uint32
	switch vk {
	case vkLButton:
		down, release = mouseeventfLeftDown, mouseeventfLeftUp
	case vkRButton:
		down, release = mouseeventfRightDown, mouseeventfRightUp
	case vkMButton:
		down, release = mouseeventfMiddleDown, mouseeventfMiddleUp
	default:
		return mouseButtonInput{}, false
	}
	flags := down
	if up {
		flags = release
	}
	return mouseButtonInput{typ: inputMouse, mi: mouseInput{flags: flags}}, true
}

// textInputs turns text into KEYEVENTF_UNICODE press/release pairs, one per
// UTF-16 code unit, so it is typed verbatim whatever the active IME. Line
// breaks become the Enter key because many programs ignore a Unicode CR/LF.
func textInputs(text string) []keyInput {
	var in []keyInput
	prevCR := false
	for _, r := range text {
		if r == '\r' || r == '\n' {
			if !(r == '\n' && prevCR) {
				in = append(in,
					keyInputFor(keymap.VKEnter, 0, false),
					keyInputFor(keymap.VKEnter, 0, true))
			}
			prevCR = r == '\r'
			continue
		}
		prevCR = false
		for _, u := range utf16.Encode([]rune{r}) {
			in = append(in,
				keyInput{typ: inputKeyboard, ki: keybdInput{scan: u, flags: keyeventfUnicode}},
				keyInput{typ: inputKeyboard, ki: keybdInput{scan: u, flags: keyeventfUnicode | keyeventfKeyUp}})
		}
	}
	return in
}

func scanCode(vk int) uint16 {
	r, _, _ := procMapVirtualKeyW.Call(uintptr(vk), mapvkVKToVSC)
	return uint16(r)
}

func send(first unsafe.Pointer, n int, size uintptr) error {
	r, _, err := procSendInput.Call(uintptr(n), uintptr(first), size)
	if int(r) != n {
		return fmt.Errorf("input: SendInput inserted %d of %d events: %w", r, n, err)
	}
	return nil
}
