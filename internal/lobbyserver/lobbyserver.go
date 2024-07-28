package lobbyserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/phuslu/log"
)

type LobbyServer struct {
	conn   *net.UDPConn
	buf    []byte
	logger *log.Logger
}

func NewLobbyServer(network, address string, logger *log.Logger) (*LobbyServer, error) {
	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("could not resolve udp addr: %w", err)
	}

	conn, err := net.ListenUDP(network, addr)
	if err != nil {
		return nil, fmt.Errorf("could not listen udp: %w", err)
	}

	// if logger is nil (which might be true in tests) => use default, but
	// silenced logger
	if logger == nil {
		tmp := log.DefaultLogger
		logger = &tmp
		logger.Writer = &log.IOWriter{Writer: io.Discard}
	}

	ls := &LobbyServer{
		conn:   conn,
		buf:    make([]byte, protocol.CmdMaxSize),
		logger: logger,
	}

	return ls, nil
}

func (ls *LobbyServer) Addr() *net.UDPAddr {
	return ls.conn.LocalAddr().(*net.UDPAddr)
}

func (ls *LobbyServer) Conn() *net.UDPConn {
	return ls.conn
}

func (ls *LobbyServer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ls.conn.Close()
		default:
			err := ls.conn.SetReadDeadline(time.Now().Add(time.Second))
			debug.Assert(err == nil)

			n, addr, err := ls.conn.ReadFromUDP(ls.buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				ls.logger.Error().
					Msgf("could not read from udp: %v", err)
				continue
			}
			if n < protocol.CmdHeaderSize {
				ls.logger.Error().
					Msgf("invalid msg size (got %d; want >= %d)", n, protocol.CmdHeaderSize)
				continue
			}

			ls.handleMsg(addr)
		}
	}
}

func (ls *LobbyServer) handleMsg(addr *net.UDPAddr) {
	cmdHeader := protocol.CmdHeader{}
	err := cmdHeader.UnmarshalBinary(ls.buf[:protocol.CmdHeaderSize])
	debug.Assert(err == nil)
	ls.logger.Debug().
		Msgf("recv: %+v", cmdHeader)

	go func(cmdHeader *protocol.CmdHeader) {
		var err error

		switch cmdHeader.Cmd {
		case protocol.CCmdPing:
			err = ls.handleCmdPing(addr)
		case protocol.CCmdJoin:
			panic("unimplemented")
		case protocol.CCmdTransformPlayer:
			panic("unimplemented")
		}

		if err != nil {
			ls.logger.Error().
				Msgf("error handling message (addr: %s; header: %v)", addr.String(), cmdHeader)
		}
	}(&cmdHeader)
}

func (ls *LobbyServer) handleCmdPing(addr *net.UDPAddr) error {
	header := protocol.CmdHeader{Cmd: protocol.SCmdPong}
	headerBytes, err := header.MarshalBinary()
	debug.Assert(err == nil)

	_, err = ls.conn.WriteToUDP(headerBytes, addr)
	return err
}
