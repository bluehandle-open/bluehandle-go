// Package keymap holds the three key tables the PC can push to the phone.
// It mirrors KeySetting.java; values are java.awt.event.KeyEvent VK_* codes
// because that is what the Android app stores and sends back.
package keymap

import (
	"fmt"

	"github.com/bluehandle-open/bluehandle-go/internal/consts"
)

// Java VK codes that are not plain ASCII letters or digits.
const (
	VKShift   = 16
	VKControl = 17
	VKAlt     = 18
	VKEscape  = 27
	VKLeft    = 37
	VKUp      = 38
	VKRight   = 39
	VKDown    = 40
	VKF5      = 116
	VKF6      = 117
)

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
	VKShift: "shift", VKControl: "ctrl", VKAlt: "alt", VKEscape: "esc",
	VKLeft: "left", VKUp: "up", VKRight: "right", VKDown: "down",
	VKF5: "F5", VKF6: "F6",
}

// KeyName renders a Java VK code for logs and the UI.
func KeyName(code int) string {
	if n, ok := names[code]; ok {
		return n
	}
	if (code >= 'A' && code <= 'Z') || (code >= '0' && code <= '9') {
		return string(rune(code))
	}
	return fmt.Sprintf("vk%d", code)
}
