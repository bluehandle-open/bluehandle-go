//go:build !windows

package client

import "syscall"

const (
	errRefused     = syscall.ECONNREFUSED
	errNetUnreach  = syscall.ENETUNREACH
	errHostUnreach = syscall.EHOSTUNREACH

	// Bluetooth is Windows-only; these never match a real error here.
	errTimedOut       = syscall.Errno(0x7fff0001)
	errServiceMissing = syscall.Errno(0x7fff0002)
	errHostDown       = syscall.Errno(0x7fff0003)
	errNetDown        = syscall.Errno(0x7fff0004)
	errNoService      = syscall.Errno(0x7fff0005)
)
