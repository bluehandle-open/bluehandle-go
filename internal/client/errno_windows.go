package client

import "syscall"

// Winsock error codes, which package syscall does not name.
const (
	errRefused     = syscall.Errno(10061) // WSAECONNREFUSED
	errNetUnreach  = syscall.Errno(10051) // WSAENETUNREACH
	errHostUnreach = syscall.Errno(10065) // WSAEHOSTUNREACH
)
