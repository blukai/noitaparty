package lobbyserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/blukai/noitaparty/internal/byteorder"
	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/phuslu/log"
)

type LobbyServer struct {
	conn   *net.UDPConn
	buf    []byte
	logger *log.Logger

	clients map[uint32]*net.UDPAddr
	seed    int32
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
	cmd := protocol.Cmd{}
	err := cmd.UnmarshalBinary(ls.buf)
	debug.Assert(err == nil)
	ls.logger.Debug().
		Msgf("recv: %+v", cmd)

	go func(cmd protocol.Cmd) {
		var err error

		switch cmd.Header.Cmd {
		case protocol.CCmdPing:
			err = ls.handleCCmdPing(addr)
		case protocol.CCmdJoin:
			err = ls.handleCCmdJoin(addr, &cmd)
		case protocol.CCmdTransformPlayer:
			err = ls.handleCCmdTransformPlayer(addr, &cmd)
		}

		if err != nil {
			ls.logger.Error().
				Msgf("error handling message (addr: %s; cmd: %v)", addr.String(), cmd)
		}
	}(cmd)
}

func (ls *LobbyServer) handleCCmdPing(addr *net.UDPAddr) error {
	header := protocol.CmdHeader{Cmd: protocol.SCmdPong}
	headerBytes, err := header.MarshalBinary()
	debug.Assert(err == nil)

	_, err = ls.conn.WriteToUDP(headerBytes, addr)
	return err
}

func id(addr *net.UDPAddr) uint32 {
	// NOTE(blukai): IPv4 constructor sets the last 4 bytes, see
	// https://cs.opensource.google/go/go/+/refs/tags/go1.22.5:src/net/ip.go;l=52
	return byteorder.Ntohl(addr.IP[12:16])
}

func (ls *LobbyServer) handleCCmdJoin(addr *net.UDPAddr, cmd *protocol.Cmd) error {
	debug.Assert(cmd.Header.Cmd == protocol.CCmdJoin)

	seed, ok := cmd.Body.(*protocol.NetworkedInt32)
	debug.Assert(ok)

	if len(ls.clients) == 0 {
		ls.seed = int32(*seed)
	}

	ls.clients[id(addr)] = addr

	return nil
}

func (ls *LobbyServer) handleCCmdTransformPlayer(addr *net.UDPAddr, cmd *protocol.Cmd) error {
	debug.Assert(cmd.Header.Cmd == protocol.CCmdTransformPlayer)

	transform, ok := cmd.Body.(*protocol.NetworkedInt32Vector2)
	debug.Assert(ok)

	senderID := id(addr)

	sendCmd := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.SCmdTransformPlayer,
			Size: 12,
		},
		Body: &protocol.NetworkedPlayer{
			ID:        protocol.NetworkedUint32(senderID),
			Transform: *transform,
		},
	}
	sendCmdBytes, err := sendCmd.MarshalBinary()
	debug.Assert(err == nil)

	for recverID, recverAddr := range ls.clients {
		if recverID == senderID {
			continue
		}

		_, err := ls.conn.WriteToUDP(sendCmdBytes, recverAddr)
		if err != nil {
			// TODO: accumulate errors, and maybe remove clients who
			// we are failing to write to!
		}
	}

	return nil
}
