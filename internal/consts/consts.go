// Package consts holds the wire-level constants shared with the Android app.
// They mirror IBluetoothConst.java in bluehandle-pc and IBlueToothConst.java
// in the Android project; do not change a value without changing both sides.
package consts

const (
	// ServerSign is the first line a Bluetooth server sends. The Wi-Fi path
	// never sends it; it is kept for completeness.
	ServerSign = "MyBluetoothApp"

	// Port is the TCP port the phone listens on.
	Port = 8088

	// MaxMessageLen bounds totalLen. The Java sides store it in a signed byte,
	// so anything above 127 would be read as negative there.
	MaxMessageLen = 127
)

// Message types. The byte sits first in phone->PC frames and second in
// PC->phone frames; see docs/protocol.md.
const (
	TypeServerSign   byte = 0x01
	TypeKey          byte = 0x02 // phone -> PC: key press/release
	TypeHandleKeySet byte = 0x10 // PC -> phone: gamepad key table
	TypePptKeySet    byte = 0x20 // PC -> phone: slideshow key table
	TypePlayerKeySet byte = 0x30 // PC -> phone: media player key table
	TypeFinish       byte = 0x40 // either side: peer is going away
	TypeHeartBeatReq byte = 0x41 // phone -> PC every second on Wi-Fi
	TypeHeartBeatRsp byte = 0x42 // defined on both sides, never sent
	TypeText         byte = 0x50 // reserved for voice input text; not implemented yet
)

// Flags in the first body byte of a TypeKey message. They may be combined.
const (
	PressFlagDown byte = 1
	PressFlagUp   byte = 2
)
