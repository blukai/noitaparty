package main

import "C"

import (
	"fmt"
	"net"
	"time"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
)

var (
	conn    *net.UDPConn
	lastErr error
)

//export LastErr
func LastErr() *C.char {
	if lastErr == nil {
		return nil
	}
	return C.CString(lastErr.Error())
}

//export Connect
func Connect(network, address *C.char) {
	addr, err := net.ResolveUDPAddr(C.GoString(network), C.GoString(address))
	if err != nil {
		lastErr = fmt.Errorf("could not resolve udp addr: %w", err)
		return
	}

	conn, err = net.DialUDP(C.GoString(network), nil, addr)
	if err != nil {
		lastErr = fmt.Errorf("could not dial udp: %w", err)
		return
	}
}

//export Disconnect
func Disconnect() {
	debug.Assert(conn != nil)
	debug.Assert(lastErr == nil)

	err := conn.Close()
	if err != nil {
		lastErr = fmt.Errorf("could not close: %w", err)
	}
}

// TODO: non-blocking send command (goroutine?)

//export SendPing
func SendPing() {
	debug.Assert(conn != nil)
	debug.Assert(lastErr == nil)

	pingHeader := protocol.CmdHeader{Cmd: protocol.CCmdPing}
	pingHeaderBytes, err := pingHeader.MarshalBinary()
	debug.Assert(err == nil)

	err = conn.SetWriteDeadline(time.Now().Add(time.Second))
	debug.Assert(err == nil)

	_, err = conn.Write(pingHeaderBytes)
	if err != nil {
		lastErr = fmt.Errorf("could not write: %w", err)
	}
}

func main() {}
