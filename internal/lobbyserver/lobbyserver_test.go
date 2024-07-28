package lobbyserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/blukai/noitaparty/internal/lobbyserver"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/matryer/is"
)

func TestPing(t *testing.T) {
	is := is.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lobbyServer, err := lobbyserver.NewLobbyServer("udp4", ":0", nil)
	is.NoErr(err)
	go lobbyServer.Run(ctx)

	clientConn, err := net.DialUDP("udp4", nil, lobbyServer.Addr())
	is.NoErr(err)
	defer clientConn.Close()

	// send ping

	pingHeader := protocol.CmdHeader{Cmd: protocol.CCmdPing}
	pingHeaderBytes, err := pingHeader.MarshalBinary()
	is.NoErr(err)

	err = clientConn.SetWriteDeadline(time.Now().Add(time.Second))
	is.NoErr(err)
	_, err = clientConn.Write(pingHeaderBytes)
	is.NoErr(err)

	// receive pong

	err = clientConn.SetReadDeadline(time.Now().Add(time.Second))
	is.NoErr(err)
	pongHeaderBytes := make([]byte, protocol.CmdHeaderSize)
	_, _, err = clientConn.ReadFromUDP(pongHeaderBytes)
	is.NoErr(err)

	pongHeader := protocol.CmdHeader{}
	err = pongHeader.UnmarshalBinary(pongHeaderBytes)
	is.NoErr(err)
	is.True(pongHeader.Cmd == protocol.SCmdPong)
}
