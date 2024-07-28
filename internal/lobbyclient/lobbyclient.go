package lobbyclient

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/blukai/noitaparty/internal/ptr"
)

type LobbyClient struct {
	conn    *net.UDPConn
	readBuf []byte

	write chan protocol.Cmd
	read  chan protocol.Cmd

	writeTimeout time.Duration
	readTimeout  time.Duration

	// NOTE(blukai): key is player's id
	players map[protocol.NetworkedUint32]*protocol.NetworkedPlayer
}

func NewLobbyClient(network, address string) (*LobbyClient, error) {
	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("could not resolve udp addr: %w", err)
	}

	conn, err := net.DialUDP(network, nil, addr)
	if err != nil {
		return nil, fmt.Errorf("could not dial udp: %w", err)
	}

	lc := &LobbyClient{
		conn:    conn,
		readBuf: make([]byte, protocol.CmdMaxSize),

		write: make(chan protocol.Cmd),
		read:  make(chan protocol.Cmd),

		writeTimeout: time.Second,
		readTimeout:  time.Second,
	}

	return lc, nil
}

func (ls *LobbyClient) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ls.conn.Close()
		case cmd := <-ls.write:
			cmdBytes, err := cmd.MarshalBinary()
			debug.Assert(err == nil)

			err = ls.conn.SetWriteDeadline(time.Now().Add(ls.writeTimeout))
			debug.Assert(err == nil)

			_, err = ls.conn.Write(cmdBytes)
			if err != nil {
				// TODO(blukai): how to handle write error?
			}
		default:
			err := ls.conn.SetReadDeadline(time.Now().Add(ls.readTimeout))
			debug.Assert(err == nil)

			n, _, err := ls.conn.ReadFromUDP(ls.readBuf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				// TODO(blukai): how to handle read error?
				continue
			}
			if n < protocol.CmdHeaderSize {
				// TODO(blukai): how to handle invalid message size error?
				continue
			}

			cmd := protocol.Cmd{}
			if err := cmd.UnmarshalBinary(ls.readBuf[0:n]); err != nil {
				// TODO(blukai): how to handle cmd unmarshal error?
				continue
			}

			switch cmd.Header.Cmd {
			// intercept some commands that don't need to be read
			// individually
			case protocol.SCmdTransformPlayer:
				player, ok := cmd.Body.(*protocol.NetworkedPlayer)
				debug.Assert(ok)
				ls.players[player.ID] = player
			default:
				ls.read <- cmd
			}

		}
	}
}

func (ls *LobbyClient) send(cmd protocol.Cmd) {
	ls.write <- cmd
}

func (ls *LobbyClient) recv() (*protocol.Cmd, error) {
	select {
	case <-time.After(ls.readTimeout):
		return nil, fmt.Errorf("timeout reached")
	case cmd := <-ls.read:
		return &cmd, nil
	}
}

func (ls *LobbyClient) SendCCmdPing() {
	sendCmd := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd: protocol.CCmdPing,
		},
	}
	ls.send(sendCmd)

	recvCmd, err := ls.recv()
	if err != nil {
		// TODO(blukai): how to handle recv error?
		return
	}
	if recvCmd.Header.Cmd != protocol.SCmdPong {
		// TODO(blukai): how to handle unexpected recv cmd error?
		return
	}
}

func (ls *LobbyClient) SendCCmdJoinRecvSCmdSetSeed(seed int32) int32 {
	sendCmd := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.CCmdJoin,
			Size: 4,
		},
		Body: ptr.To(protocol.NetworkedInt32(seed)),
	}
	ls.send(sendCmd)

	recvCmd, err := ls.recv()
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

func (ls *LobbyClient) SendSCmdTransformPlayer(x int32, y int32) {
	sendCmd := protocol.Cmd{
		Header: &protocol.CmdHeader{
			Cmd:  protocol.CCmdTransformPlayer,
			Size: 8,
		},
		Body: ptr.To(protocol.NetworkedInt32Vector2{
			X: protocol.NetworkedInt32(x),
			Y: protocol.NetworkedInt32(y),
		}),
	}
	ls.send(sendCmd)
}

// TODO(blukai): GetDeltaPlayers or something.. to not have to
// re-draw(/re-update) things that already are up to date.
func (ls *LobbyClient) GetPlayers() []*protocol.NetworkedPlayer {
	nel := len(ls.players)
	players := make([]*protocol.NetworkedPlayer, nel, nel)
	i := 0
	for _, player := range ls.players {
		players[i] = player
		i += 1
	}
	return players
}
