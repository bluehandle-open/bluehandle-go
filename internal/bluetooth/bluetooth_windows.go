package bluetooth

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// sppUUID is the serial-port profile the phone registers its service under:
// 00001101-0000-1000-8000-00805F9B34FB.
var sppUUID = windows.GUID{
	Data1: 0x00001101, Data2: 0x0000, Data3: 0x1000,
	Data4: [8]byte{0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB},
}

var (
	bthprops            = windows.NewLazySystemDLL("bthprops.cpl")
	procFindFirstRadio  = bthprops.NewProc("BluetoothFindFirstRadio")
	procFindRadioClose  = bthprops.NewProc("BluetoothFindRadioClose")
	procFindFirstDevice = bthprops.NewProc("BluetoothFindFirstDevice")
	procFindNextDevice  = bthprops.NewProc("BluetoothFindNextDevice")
	procFindDeviceClose = bthprops.NewProc("BluetoothFindDeviceClose")
)

// radioParams is BLUETOOTH_FIND_RADIO_PARAMS.
type radioParams struct{ size uint32 }

// searchParams is BLUETOOTH_DEVICE_SEARCH_PARAMS.
type searchParams struct {
	size                uint32
	returnAuthenticated int32
	returnRemembered    int32
	returnUnknown       int32
	returnConnected     int32
	issueInquiry        int32
	timeoutMultiplier   uint8
	_                   [3]byte
	radio               windows.Handle
}

// deviceInfo is BLUETOOTH_DEVICE_INFO. The address union is 8-byte aligned
// in C on every architecture, hence the explicit padding.
type deviceInfo struct {
	size          uint32
	_             uint32
	address       uint64
	classOfDevice uint32
	connected     int32
	remembered    int32
	authenticated int32
	lastSeen      windows.Systemtime
	lastUsed      windows.Systemtime
	name          [248]uint16
}

// Paired lists the devices Windows has paired with, phones first. It
// returns ErrNoRadio when there is no adapter or Bluetooth is turned off.
func Paired() ([]Device, error) {
	if err := bthprops.Load(); err != nil {
		return nil, ErrUnsupported
	}
	rp := radioParams{size: uint32(unsafe.Sizeof(radioParams{}))}
	var radio windows.Handle
	find, _, _ := procFindFirstRadio.Call(uintptr(unsafe.Pointer(&rp)), uintptr(unsafe.Pointer(&radio)))
	if find == 0 {
		return nil, ErrNoRadio
	}
	procFindRadioClose.Call(find)
	windows.CloseHandle(radio)

	sp := searchParams{
		size:                uint32(unsafe.Sizeof(searchParams{})),
		returnAuthenticated: 1,
		returnRemembered:    1,
		returnConnected:     1,
	}
	di := deviceInfo{size: uint32(unsafe.Sizeof(deviceInfo{}))}
	h, _, err := procFindFirstDevice.Call(uintptr(unsafe.Pointer(&sp)), uintptr(unsafe.Pointer(&di)))
	if h == 0 {
		if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
			return []Device{}, nil
		}
		return nil, err
	}
	defer procFindDeviceClose.Call(h)

	var out []Device
	for {
		if di.authenticated != 0 || di.remembered != 0 {
			out = append(out, Device{
				Name:      windows.UTF16ToString(di.name[:]),
				Addr:      FormatAddr(di.address),
				Phone:     (di.classOfDevice>>8)&0x1F == 2, // major class: phone
				Connected: di.connected != 0,
			})
		}
		di = deviceInfo{size: uint32(unsafe.Sizeof(deviceInfo{}))}
		if ok, _, _ := procFindNextDevice.Call(h, uintptr(unsafe.Pointer(&di))); ok == 0 {
			break
		}
	}
	sortDevices(out)
	return out, nil
}

// Dial opens an RFCOMM connection to the serial-port service on address.
// Windows looks the channel up through SDP, so the phone must be paired and
// listening. Cancelling ctx aborts a pending connect.
func Dial(ctx context.Context, address string) (net.Conn, error) {
	bt, err := ParseAddr(address)
	if err != nil {
		return nil, err
	}
	fd, err := windows.Socket(windows.AF_BTH, windows.SOCK_STREAM, windows.BTHPROTO_RFCOMM)
	if err != nil {
		if errors.Is(err, windows.WSAEAFNOSUPPORT) {
			return nil, ErrNoRadio
		}
		return nil, os.NewSyscallError("socket", err)
	}
	c := &conn{fd: fd, remote: addr(bt)}

	done := make(chan error, 1)
	go func() {
		done <- windows.Connect(fd, &windows.SockaddrBth{BtAddr: bt, ServiceClassId: sppUUID})
	}()
	select {
	case err = <-done:
	case <-ctx.Done():
		c.Close() // unblocks connect
		<-done
		return nil, ctx.Err()
	}
	if err != nil {
		c.Close()
		return nil, os.NewSyscallError("connect", err)
	}
	return c, nil
}

// conn is a blocking RFCOMM socket. Deadlines map to SO_RCVTIMEO and
// SO_SNDTIMEO, which is enough for the client: it sets one deadline before
// each read or write and drops the connection when one expires.
type conn struct {
	fd     windows.Handle
	remote addr
	once   sync.Once
}

func (c *conn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := windows.WSABuf{Len: uint32(len(p)), Buf: &p[0]}
	var n, flags uint32
	if err := windows.WSARecv(c.fd, &buf, 1, &n, &flags, nil, nil); err != nil {
		return 0, c.opError("read", err)
	}
	if n == 0 {
		return 0, net.ErrClosed
	}
	return int(n), nil
}

func (c *conn) Write(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		buf := windows.WSABuf{Len: uint32(len(p) - written), Buf: &p[written]}
		var n uint32
		if err := windows.WSASend(c.fd, &buf, 1, &n, 0, nil, nil); err != nil {
			return written, c.opError("write", err)
		}
		written += int(n)
	}
	return written, nil
}

func (c *conn) opError(op string, err error) error {
	if errors.Is(err, windows.WSAETIMEDOUT) {
		err = os.ErrDeadlineExceeded
	}
	return &net.OpError{Op: op, Net: "bluetooth", Addr: c.remote, Err: err}
}

func (c *conn) Close() error {
	var err error
	c.once.Do(func() { err = windows.Closesocket(c.fd) })
	return err
}

func (c *conn) LocalAddr() net.Addr  { return addr(0) }
func (c *conn) RemoteAddr() net.Addr { return c.remote }

func (c *conn) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}
func (c *conn) SetReadDeadline(t time.Time) error  { return c.timeout(windows.SO_RCVTIMEO, t) }
func (c *conn) SetWriteDeadline(t time.Time) error { return c.timeout(windows.SO_SNDTIMEO, t) }

func (c *conn) timeout(opt int, t time.Time) error {
	ms := 0 // zero time: no timeout
	if !t.IsZero() {
		ms = int(time.Until(t) / time.Millisecond)
		if ms < 1 {
			ms = 1
		}
	}
	if err := windows.SetsockoptInt(c.fd, windows.SOL_SOCKET, opt, ms); err != nil {
		return os.NewSyscallError("setsockopt", err)
	}
	return nil
}
