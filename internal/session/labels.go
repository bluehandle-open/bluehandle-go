package session

import "github.com/bluehandle-open/bluehandle-go/internal/keymap"

// labels names keys the way the phone's buttons and the design call them.
var labels = map[int]string{
	keymap.VKBack: "退格", keymap.VKTab: "Tab", keymap.VKEnter: "Enter",
	keymap.VKShift: "Shift", keymap.VKControl: "Ctrl", keymap.VKAlt: "Alt",
	keymap.VKEscape: "Esc", keymap.VKSpace: "空格",
	keymap.VKPageUp: "PgUp", keymap.VKPageDown: "PgDn",
	keymap.VKLeft: "←", keymap.VKUp: "↑", keymap.VKRight: "→", keymap.VKDown: "↓",
	keymap.VKInsert: "Insert", keymap.VKDelete: "Delete",
	keymap.VKLWin: "Win", keymap.VKApps: "菜单键",
	keymap.VKVolumeMute: "静音", keymap.VKVolumeDown: "音量-", keymap.VKVolumeUp: "音量+",
	keymap.VKMediaNext: "下一曲", keymap.VKMediaPrev: "上一曲",
	keymap.VKMediaStop: "停止", keymap.VKMediaPlayPause: "播放/暂停",
	keymap.VKOemMinus: "-", keymap.VKOemPeriod: ".",
	0x01: "鼠标左键", 0x02: "鼠标右键", 0x04: "鼠标中键",
	0x23: "End", 0x24: "Home", 0x14: "CapsLock",
	0xBA: ";", 0xBB: "=", 0xBC: ",", 0xBF: "/", 0xC0: "`",
	0xDB: "[", 0xDC: "\\", 0xDD: "]", 0xDE: "'",
}

// Label names one Windows virtual-key code for the activity log.
func Label(vk int) string {
	if l, ok := labels[vk]; ok {
		return l
	}
	switch {
	case vk >= 0x70 && vk <= 0x87:
		return "F" + itoa(vk-0x6F)
	case vk >= 0x60 && vk <= 0x69:
		return "小键盘" + itoa(vk-0x60)
	}
	return keymap.KeyName(vk)
}

// Labels names each code in order.
func Labels(vks []int) []string {
	out := make([]string, len(vks))
	for i, vk := range vks {
		out[i] = Label(vk)
	}
	return out
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
