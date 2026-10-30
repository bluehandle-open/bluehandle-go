// Package keymap holds the key code convention shared with the phone and the
// three key tables the PC can push to it (KeySetting.java).
//
// Key codes on the wire are Windows virtual-key codes (VK_*). The Android
// app's custom key table mixes in a few non-Windows values; Normalize maps
// those so injectors only ever see Windows codes.
package keymap

import (
	"fmt"

	"github.com/bluehandle-open/bluehandle-go/internal/consts"
)

// Windows virtual-key codes that are not plain ASCII letters or digits.
const (
	VKBack     = 0x08
	VKTab      = 0x09
	VKEnter    = 0x0D
	VKShift    = 0x10
	VKControl  = 0x11
	VKAlt      = 0x12
	VKEscape   = 0x1B
	VKSpace    = 0x20
	VKPageUp   = 0x21
	VKPageDown = 0x22
	VKLeft     = 0x25
	VKUp       = 0x26
	VKRight    = 0x27
	VKDown     = 0x28
	VKInsert   = 0x2D
	VKDelete   = 0x2E
	VKLWin     = 0x5B
	VKApps     = 0x5D // context menu key
	VKF5       = 0x74
	VKF6       = 0x75

	VKVolumeMute     = 0xAD
	VKVolumeDown     = 0xAE
	VKVolumeUp       = 0xAF
	VKMediaNext      = 0xB0
	VKMediaPrev      = 0xB1
	VKMediaStop      = 0xB2
	VKMediaPlayPause = 0xB3

	VKOemMinus  = 0xBD // '-' on the main keyboard
	VKOemPeriod = 0xBE // '.' on the main keyboard
)

// legacy maps codes from the Android app's original key table that are not
// Windows codes. 0x7F is VK_F16 on Windows but the app uses it for Delete,
// which is by far the more useful key. Codes 59-64 (; < = > ? @) are ASCII
// and undefined as Windows VKs, so they pass through for injectors to type
// as characters.
var legacy = map[int]int{
	45:  VKOemMinus, // Windows VK_INSERT, but the app sends 45 for '-'
	127: VKDelete,
	155: VKInsert, // Java VK_INSERT
}

// Normalize converts a wire key code into a Windows virtual-key code.
func Normalize(code int) int {
	if vk, ok := legacy[code]; ok {
		return vk
	}
	return code
}

// Handle is the gamepad table in the order the Java PC client sends it.
// Letters and digits use their upper-case ASCII value, which equals the VK code.
var Handle = []byte{'W', 'S', 'A', 'D', 'U', 'I', 'O', 'J', 'K', 'L', '1', '5'}

// Ppt is the slideshow table: up, down, left, right, escape, shift+F5.
var Ppt = []byte{VKUp, VKDown, VKLeft, VKRight, VKEscape, VKShift, VKF5}

// Player is the media player table, six ctrl+alt chords:
// play/pause, stop, previous, next, volume up, volume down.
var Player = []byte{
	VKControl, VKAlt, VKF5,
	VKControl, VKAlt, VKF6,
	VKControl, VKAlt, VKLeft,
	VKControl, VKAlt, VKRight,
	VKControl, VKAlt, VKUp,
	VKControl, VKAlt, VKDown,
}

// ByType returns a copy of the table for a Type*KeySet message type.
func ByType(typ byte) ([]byte, bool) {
	var src []byte
	switch typ {
	case consts.TypeHandleKeySet:
		src = Handle
	case consts.TypePptKeySet:
		src = Ppt
	case consts.TypePlayerKeySet:
		src = Player
	default:
		return nil, false
	}
	return append([]byte(nil), src...), true
}

// TypeByName maps the CLI/UI names "handle", "ppt" and "player" to message types.
func TypeByName(name string) (byte, bool) {
	switch name {
	case "handle":
		return consts.TypeHandleKeySet, true
	case "ppt":
		return consts.TypePptKeySet, true
	case "player":
		return consts.TypePlayerKeySet, true
	}
	return 0, false
}

var names = map[int]string{
	VKBack: "backspace", VKTab: "tab", VKEnter: "enter",
	VKShift: "shift", VKControl: "ctrl", VKAlt: "alt", VKEscape: "esc", VKSpace: "space",
	VKPageUp: "pageup", VKPageDown: "pagedown",
	VKLeft: "left", VKUp: "up", VKRight: "right", VKDown: "down",
	VKInsert: "insert", VKDelete: "delete", VKLWin: "win", VKApps: "menu",
	VKF5: "F5", VKF6: "F6",
	VKVolumeMute: "mute", VKVolumeDown: "volumedown", VKVolumeUp: "volumeup",
	VKMediaNext: "next", VKMediaPrev: "prev", VKMediaStop: "stop", VKMediaPlayPause: "playpause",
	VKOemMinus: "-", VKOemPeriod: ".",
}

// KeyName renders a Windows VK code (after Normalize) for logs and the UI.
func KeyName(code int) string {
	if n, ok := names[code]; ok {
		return n
	}
	if (code >= 'A' && code <= 'Z') || (code >= '0' && code <= '9') || (code >= ';' && code <= '@') {
		return string(rune(code))
	}
	return fmt.Sprintf("vk%d", code)
}
