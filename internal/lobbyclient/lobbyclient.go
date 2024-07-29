package lobbyclient

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

type LobbyClient struct {
	conn    *net.UDPConn
	readBuf []byte

	logger *log.Logger

	sendCh chan protocol.Cmd
	recvCh chan protocol.Cmd

	writeTimeout time.Duration
	readTimeout  time.Duration

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

		sendCh: make(chan protocol.Cmd),
		recvCh: make(chan protocol.Cmd),

		writeTimeout: time.Second,
		readTimeout:  time.Second,

		players: make(map[protocol.NetworkedUint64]*protocol.NetworkedTransformPlayer),
	}

	return lc, nil
}

func (lc *LobbyClient) Run(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cCmdHeartbeat := protocol.Cmd{
					Header: &protocol.CmdHeader{
						Cmd:  protocol.CCmdHeartbeat,
						Size: 0,
					},
					Body: nil,
				}
				lc.sendCmd(cCmdHeartbeat)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			lc.logger.Debug().
				Msg("done")
			return lc.conn.Close()
		case cmd := <-lc.sendCh:
			cmdBytes, err := cmd.MarshalBinary()
			debug.Assert(err == nil)

			err = lc.conn.SetWriteDeadline(time.Now().Add(lc.writeTimeout))
			debug.Assert(err == nil)

			_, err = lc.conn.Write(cmdBytes)
			if err != nil {
				lc.logger.Error().
					Msgf("could not write: %v", err)
				// TODO(blukai): how to handle write error?
			}
		default:
			err := lc.conn.SetReadDeadline(time.Now().Add(lc.readTimeout))
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
				Msgf("recv: %+#v", &cmd)

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

func (lc *LobbyClient) sendCmd(cmd protocol.Cmd) {
	lc.sendCh <- cmd
}

func (lc *LobbyClient) recvCmd() (*protocol.Cmd, error) {
	select {
	case <-time.After(lc.readTimeout):
		return nil, fmt.Errorf("timeout reached")
	case cmd := <-lc.recvCh:
		return &cmd, nil
	}
}

func (lc *LobbyClient) SendCCmdPing() {
	cCmdPing := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd: protocol.CCmdPing,
		},
	}
	lc.sendCmd(cCmdPing)

	sCmdPong, err := lc.recvCmd()
	if err != nil {
		// TODO(blukai): how to handle recv error?
		return
	}
	if sCmdPong.Header.Cmd != protocol.SCmdPong {
		// TODO(blukai): how to handle unexpected recv cmd error?
		return
	}
}

// SendCCmdJoinRecvSCmdSetSeed returns seed. 0 is invalid value.
func (lc *LobbyClient) SendCCmdJoinRecvSCmdSetSeed(id uint64, seed int32) int32 {
	cCmdJoin := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.CCmdJoin,
			Size: 12,
		},
		Body: &protocol.NetworkedJoin{
			ID:   protocol.NetworkedUint64(id),
			Seed: protocol.NetworkedInt32(seed),
		},
	}
	lc.sendCmd(cCmdJoin)

	recvCmd, err := lc.recvCmd()
	if err != nil {
		// TODO(blukai): how to handle recv error?
		return 0
	}
	if recvCmd.Header.Cmd != protocol.SCmdSetSeed {
		// TODO(blukai): how to handle unexpected recv cmd error?
		return 0
	}

	recvSeed, ok := recvCmd.Body.(*protocol.NetworkedInt32)
	debug.Assert(ok)

	return int32(*recvSeed)
}

func (lc *LobbyClient) SendSCmdTransformPlayer(id uint64, x int32, y int32) {
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
