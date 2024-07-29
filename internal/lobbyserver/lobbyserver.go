package lobbyserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/blukai/noitaparty/internal/ptr"
	"github.com/hashicorp/go-multierror"
	"github.com/phuslu/log"
)

type client struct {
	addr          *net.UDPAddr
	lastHeartbeat time.Time
}

type LobbyServer struct {
	conn *net.UDPConn
	buf  []byte

	logger *log.Logger

	clients map[uint64]*client
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
		conn: conn,
		buf:  make([]byte, protocol.CmdMaxSize),

		logger: logger,

		clients: make(map[uint64]*client),
		seed:    0,
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
	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				for clientID, client := range ls.clients {
					if now.Sub(client.lastHeartbeat) > time.Second*10 {
						delete(ls.clients, clientID)
					}
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			ls.logger.Debug().
				Msg("done")
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
		Msgf("recv: %+#v", &cmd)

	go func(cmd protocol.Cmd) {
		var err error

		switch cmd.Header.Cmd {
		case protocol.CCmdPing:
			err = ls.handleCCmdPing(addr)
		case protocol.CCmdJoin:
			err = ls.handleCCmdJoin(&cmd, addr)
		case protocol.CCmdTransformPlayer:
			err = ls.handleCCmdTransformPlayer(&cmd)
		}

		if err != nil {
			ls.logger.Error().
				Msgf("error handling message (addr: %s; cmd: %v)", addr.String(), cmd)
		}
	}(cmd)
}

func (ls *LobbyServer) sendBytes(bytes []byte, addr *net.UDPAddr) error {
	ls.logger.Debug().
		Str("bytes", fmt.Sprintf("%v", bytes)).
		Msg("sendBytes")
	_, err := ls.conn.WriteToUDP(bytes, addr)
	return err
}

func (ls *LobbyServer) sendCmd(cmd protocol.Cmd, addr *net.UDPAddr) error {
	ls.logger.Debug().
		Any("cmd", &cmd).
		Any("addr", addr).
		Msg("sendCmd")

	bytes, err := cmd.MarshalBinary()
	debug.Assert(err == nil)

	return ls.sendBytes(bytes, addr)
}

func (ls *LobbyServer) handleCCmdPing(addr *net.UDPAddr) error {
	sCmdPong := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.SCmdPong,
			Size: 0,
		},
		Body: nil,
	}
	return ls.sendCmd(sCmdPong, addr)
}

func (ls *LobbyServer) handleCCmdJoin(cCmdJoin *protocol.Cmd, addr *net.UDPAddr) error {
	debug.Assert(cCmdJoin.Header.Cmd == protocol.CCmdJoin)

	join, ok := cCmdJoin.Body.(*protocol.NetworkedJoin)
	debug.Assert(ok)

	// first who joins sets the seed (for now at least)
	if len(ls.clients) == 0 {
		ls.seed = int32(join.Seed)
	}

	ls.clients[uint64(join.ID)] = &client{
		addr:          addr,
		lastHeartbeat: time.Now(),
	}

	sCmdSetSeed := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.SCmdSetSeed,
			Size: 4,
		},
		Body: ptr.To(protocol.NetworkedInt32(ls.seed)),
	}
	return ls.sendCmd(sCmdSetSeed, addr)
}

func (ls *LobbyServer) handleCCmdTransformPlayer(cCmdTransformPlayer *protocol.Cmd) error {
	debug.Assert(cCmdTransformPlayer.Header.Cmd == protocol.CCmdTransformPlayer)

	transformPlayer, ok := cCmdTransformPlayer.Body.(*protocol.NetworkedTransformPlayer)
	debug.Assert(ok)

	sCmdTransformPlayer := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.SCmdTransformPlayer,
			Size: 16,
		},
		Body: transformPlayer,
	}
	sCmdTransformPlayerBytes, err := sCmdTransformPlayer.MarshalBinary()
	debug.Assert(err == nil)

	var errs error
	for clientID, client := range ls.clients {
		// don't send to the sender
		if clientID == uint64(transformPlayer.ID) {
			continue
		}

		err := ls.sendBytes(sCmdTransformPlayerBytes, client.addr)
		if err != nil {
			ls.logger.Error().
				Msgf("could not send player transform to %v: %v", client, err)

			errs = multierror.Append(errs, err)
		}
	}
	return errs
}
