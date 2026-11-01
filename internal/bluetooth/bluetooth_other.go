//go:build !windows

package bluetooth

import (
	"context"
	"net"
)

// Paired is only implemented on Windows.
func Paired() ([]Device, error) { return nil, ErrUnsupported }

// Dial is only implemented on Windows.
func Dial(ctx context.Context, address string) (net.Conn, error) { return nil, ErrUnsupported }
