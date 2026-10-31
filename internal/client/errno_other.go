//go:build !windows

package client

import "syscall"

const (
	errRefused     = syscall.ECONNREFUSED
	errNetUnreach  = syscall.ENETUNREACH
	errHostUnreach = syscall.EHOSTUNREACH
)
