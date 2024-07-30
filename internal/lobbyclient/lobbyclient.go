package lobbyclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/blukai/noitaparty/internal/ptr"
	"github.com/phuslu/log"
)

type sendChPayload struct {
	cmd   protocol.Cmd
	errCh chan error
}

type LobbyClient struct {
	conn    *net.UDPConn
	readBuf []byte

	logger *log.Logger

	sendCh chan sendChPayload
	recvCh chan protocol.Cmd

	sendTimeout time.Duration
	recvTimeout time.Duration

	// NOTE(blukai): key is player's id
	players map[protocol.NetworkedUint64]*protocol.NetworkedTransformPlayer
}

func NewLobbyClient(network, address string, logger *log.Logger) (*LobbyClient, error) {
	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("could not resolve udp addr: %w", err)
	}

	conn, err := net.DialUDP(network, nil, addr)
	if err != nil {
		return nil, fmt.Errorf("could not dial udp: %w", err)
	}

	// if logger is nil (which might be true in tests) => use default, but
	// silenced logger
	if logger == nil {
		tmp := log.DefaultLogger
		logger = &tmp
		logger.Writer = &log.IOWriter{Writer: io.Discard}
	}

	lc := &LobbyClient{
		conn:    conn,
		readBuf: make([]byte, protocol.CmdMaxSize),

		logger: logger,

		sendCh: make(chan sendChPayload),
		recvCh: make(chan protocol.Cmd),

		sendTimeout: time.Second,
		recvTimeout: time.Second,

		players: make(map[protocol.NetworkedUint64]*protocol.NetworkedTransformPlayer),
	}

	return lc, nil
}

func (lc *LobbyClient) runSendCh(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case payload := <-lc.sendCh:
			lc.logger.Debug().
				Any("cmd", &payload.cmd).
				Msg("sendCmd")

			cmdBytes, err := payload.cmd.MarshalBinary()
			debug.Assert(err == nil)

			err = lc.conn.SetWriteDeadline(time.Now().Add(lc.sendTimeout))
			debug.Assert(err == nil)

			_, err = lc.conn.Write(cmdBytes)
			if err != nil {
				lc.logger.Error().
					Msgf("could not write: %v", err)

				payload.errCh <- err
				continue
			}

			// TODO(blukai): do i need to send a nil, can't i just
			// close it?
			payload.errCh <- nil
			close(payload.errCh)
		}
	}
}

func (lc *LobbyClient) runRecvCh(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := lc.conn.SetReadDeadline(time.Now().Add(lc.recvTimeout))
			debug.Assert(err == nil)

			n, _, err := lc.conn.ReadFromUDP(lc.readBuf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				lc.logger.Error().
					Msgf("could not read: %v", err)

				// TODO(blukai): how to handle read error?
				continue
			}
			if n < protocol.CmdHeaderSize {
				lc.logger.Error().
					Msgf("invalid msg size (got %d; want >= %d)", n, protocol.CmdHeaderSize)
				continue
			}

			cmd := protocol.Cmd{}
			if err := cmd.UnmarshalBinary(lc.readBuf[0:n]); err != nil {
				lc.logger.Error().
					Str("bytes", fmt.Sprintf("%v", lc.readBuf[0:n])).
					Msgf("could not unmarshal cmd: %v", err)
				continue
			}

			lc.logger.Debug().
				Any("cmd", &cmd).
				Msgf("recv")

			switch cmd.Header.Cmd {
			// intercept some commands that don't need to be read
			// individually
			case protocol.SCmdTransformPlayer:
				player, ok := cmd.Body.(*protocol.NetworkedTransformPlayer)
				debug.Assert(ok)
				lc.players[player.ID] = player
			default:
				lc.recvCh <- cmd
			}
		}
	}
}

func (lc *LobbyClient) runKeepAlive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		// send keep alive messages periodically if no other messages
		// are being sent
		case <-time.After(time.Second * 5):
			sCmdKeepAlive := protocol.Cmd{
				Header: &protocol.CmdHeader{
					Cmd:  protocol.CCmdKeepAlive,
					Size: 4,
				},
				Body: nil,
			}
			lc.sendCmd(sCmdKeepAlive)
		}
	}
}

func (lc *LobbyClient) Run(ctx context.Context) error {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		lc.runSendCh(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		lc.runRecvCh(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		lc.runKeepAlive(ctx)
	}()

	select {
	case <-ctx.Done():
		wg.Wait()
		return lc.conn.Close()
	}
}

func (lc *LobbyClient) sendCmd(cmd protocol.Cmd) <-chan error {
	errChan := make(chan error, 1)
	lc.sendCh <- sendChPayload{
		cmd:   cmd,
		errCh: errChan,
	}
	return errChan
}

func (lc *LobbyClient) recvCmd() (*protocol.Cmd, error) {
	select {
	case <-time.After(lc.recvTimeout):
		return nil, fmt.Errorf("timeout reached")
	case cmd := <-lc.recvCh:
		return &cmd, nil
	}
}

// SendCCmdPing is blocking
func (lc *LobbyClient) SendCCmdPing() error {
	cCmdPing := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd: protocol.CCmdPing,
		},
	}
	err := <-lc.sendCmd(cCmdPing)
	if err != nil {
		return fmt.Errorf("could not send: %w", err)
	}

	sCmdPong, err := lc.recvCmd()
	if err != nil {
		return fmt.Errorf("could not recv: %w", err)
	}
	if sCmdPong.Header.Cmd != protocol.SCmdPong {
		return fmt.Errorf(
			"received unexpected cmd back (got %d; want %d)",
			sCmdPong.Header.Cmd,
			protocol.SCmdPong,
		)
	}

	return nil
}

// SendCCmdJoinRecvSCmdSetSeed is blocking
func (lc *LobbyClient) SendCCmdJoinRecvSCmdSetSeed(id uint64) (int32, error) {
	cCmdJoin := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.CCmdJoin,
			Size: 8,
		},
		Body: ptr.To(protocol.NetworkedUint64(id)),
	}
	err := <-lc.sendCmd(cCmdJoin)
	if err != nil {
		return 0, fmt.Errorf("could not send: %w", err)
	}

	recvCmd, err := lc.recvCmd()
	if err != nil {
		return 0, fmt.Errorf("could not recv: %w", err)
	}
	if recvCmd.Header.Cmd != protocol.SCmdSetSeed {
		// TODO(blukai): generalize unexpected cmd err
		return 0, fmt.Errorf(
			"received unexpected cmd back (got %d; want %d)",
			recvCmd.Header.Cmd,
			protocol.SCmdPong,
		)
	}

	recvSeed, ok := recvCmd.Body.(*protocol.NetworkedInt32)
	debug.Assert(ok)

	return int32(*recvSeed), nil
}

// SendCCmdTransformPlayer is non-blocking, potential err is ignored
func (lc *LobbyClient) SendCCmdTransformPlayer(id uint64, x int32, y int32) {
	cCmdTransformPlayer := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.CCmdTransformPlayer,
			Size: 16,
		},
		Body: &protocol.NetworkedTransformPlayer{
			ID: protocol.NetworkedUint64(id),
			Transform: protocol.NetworkedInt32Vector2{
				X: protocol.NetworkedInt32(x),
				Y: protocol.NetworkedInt32(y),
			},
		},
	}
	lc.sendCmd(cCmdTransformPlayer)
}

// TODO(blukai): GetDeltaPlayers or something.. to not have to
// re-draw(/re-update) things that already are up to date.
func (lc *LobbyClient) GetPlayers() []*protocol.NetworkedTransformPlayer {
	nel := len(lc.players)
	players := make([]*protocol.NetworkedTransformPlayer, nel, nel)
	i := 0
	for _, player := range lc.players {
		players[i] = player
		i += 1
	}
	return players
}
