package lobbyserver

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/blukai/noitaparty/internal/ptr"
	"github.com/cespare/xxhash/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/phuslu/log"
)

type addrKey uint64

func makeAddrKey(addr *net.UDPAddr) addrKey {
	return addrKey(xxhash.Sum64String(addr.String()))
}

type client struct {
	addr     *net.UDPAddr
	lastSeen time.Time
}

type LobbyServer struct {
	conn *net.UDPConn
	buf  []byte

	logger *log.Logger

	clients map[addrKey]*client
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

		clients: make(map[addrKey]*client),
		seed:    0,
	}

	return ls, nil
}

// Addr can be useful to retreive server's address when LobbyServer was
// constructed with ":0".
func (ls *LobbyServer) Addr() *net.UDPAddr {
	return ls.conn.LocalAddr().(*net.UDPAddr)
}

func (ls *LobbyServer) runRecv(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
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

			cmd := protocol.Cmd{}
			if err := cmd.UnmarshalBinary(ls.buf); err != nil {
				ls.logger.Error().
					Str("bytes", fmt.Sprintf("%v", ls.buf[0:n])).
					Msgf("could not unmarshal cmd: %v", err)
				continue
			}

			client, ok := ls.clients[makeAddrKey(addr)]
			// client is created in handleCCmdJoin func
			if ok {
				client.lastSeen = time.Now()
			}

			ls.logger.Debug().
				Any("cmd", &cmd).
				Any("addr", addr).
				Msgf("recv")

			// TODO(blukai): can this spawn a shit ton of go routines?
			go ls.handleCmd(cmd, addr)
		}
	}
}

// TODO(blukai): how to handle re-connects? get rid of join message? send seed
// together with probably https "authentication" response?
func (ls *LobbyServer) runClientEvictor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
			now := time.Now()
			for clientAddrKey, client := range ls.clients {
				if now.Sub(client.lastSeen) > time.Second*10 {
					delete(ls.clients, clientAddrKey)
					ls.logger.Debug().
						Str("client", fmt.Sprintf("%+#v", client)).
						Msg("evicted client")
				}
			}
		}
	}
}

func (ls *LobbyServer) Run(ctx context.Context) error {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ls.runRecv(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ls.runClientEvictor(ctx)
	}()

	select {
	case <-ctx.Done():
		wg.Wait()
		return ls.conn.Close()
	}
}

func (ls *LobbyServer) handleCmd(cmd protocol.Cmd, addr *net.UDPAddr) {
	var err error

	switch cmd.Header.Cmd {
	case protocol.CCmdPing:
		err = ls.handleCCmdPing(addr)
	case protocol.CCmdJoin:
		err = ls.handleCCmdJoin(&cmd, addr)
	case protocol.CCmdTransformPlayer:
		err = ls.handleCCmdTransformPlayer(&cmd, addr)
	case protocol.CCmdKeepAlive:
		// ignore keep alive because lastSeen is being maintained by
		// runRecv func
	default:
		debug.Assert(false, fmt.Sprintf("unhandled cmd: %d", cmd.Header.Cmd))
	}

	if err != nil {
		ls.logger.Error().
			Msgf("error handling message (addr: %s; cmd: %v)", addr.String(), cmd)
	}
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

	id, ok := cCmdJoin.Body.(*protocol.NetworkedUint64)
	debug.Assert(ok)
	// TODO(blukai): get rid of id if it'll end up not being needed
	_ = id

	// TODO(blukai): controllable seed
	//
	// for now if there are no players generate a random seed
	if len(ls.clients) == 0 {
		ls.seed = rand.Int31()
	}

	// TODO: some form of authentication (but keep stuff anonymized).
	//
	// maybe require clients to receive a token over https or in some other
	// secure way and then send it with each udp packet or/and use it to
	// encrypt messages (on client).
	ls.clients[makeAddrKey(addr)] = &client{
		addr:     addr,
		lastSeen: time.Now(),
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

func (ls *LobbyServer) handleCCmdTransformPlayer(
	cCmdTransformPlayer *protocol.Cmd,
	addr *net.UDPAddr,
) error {
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

	// broadcast to everyone else
	addrKey := makeAddrKey(addr)
	var errs error
	for clientAddrKey, client := range ls.clients {
		// don't send to the sender
		if clientAddrKey == addrKey {
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
