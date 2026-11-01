package client

import "syscall"

// Winsock error codes, which package syscall does not name.
const (
	errRefused     = syscall.Errno(10061) // WSAECONNREFUSED
	errNetUnreach  = syscall.Errno(10051) // WSAENETUNREACH
	errHostUnreach = syscall.Errno(10065) // WSAEHOSTUNREACH

	// Seen when dialling over Bluetooth.
	errTimedOut       = syscall.Errno(10060) // WSAETIMEDOUT: phone did not answer
	errServiceMissing = syscall.Errno(10108) // WSASERVICE_NOT_FOUND: phone is not waiting
	errNoService      = syscall.Errno(10049) // WSAEADDRNOTAVAIL: same, as some stacks report it
	errHostDown       = syscall.Errno(10064) // WSAEHOSTDOWN: phone out of range or off
	errNetDown        = syscall.Errno(10050) // WSAENETDOWN: this computer's Bluetooth is off
)
