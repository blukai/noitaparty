package main

import "C"

import (
	"context"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/lobbyclient"
	"github.com/blukai/noitaparty/internal/protocol"
)

var (
	lc      *lobbyclient.LobbyClient
	lastErr error
	cancel  context.CancelFunc
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
	debug.Assert(lc == nil)

	lobbyClient, err := lobbyclient.NewLobbyClient(C.GoString(network), C.GoString(address), nil)
	if err != nil {
		lastErr = err
		return
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	go lobbyClient.Run(ctx)

	if err := lobbyClient.SendCCmdPing(); err != nil {
		lastErr = err
		cancelFunc()
		return
	}

	lc = lobbyClient
	cancel = cancelFunc
}

//export SendCCmdJoinRecvSCmdSetSeed
func SendCCmdJoinRecvSCmdSetSeed(id uint64) int32 {
	debug.Assert(lc != nil)
	debug.Assert(lastErr == nil)

	seed, err := lc.SendCCmdJoinRecvSCmdSetSeed(id)
	if err != nil {
		lastErr = err
		return 0
	}

	return seed
}

//export SendCCmdTransformPlayer
func SendCCmdTransformPlayer(id uint64, x int32, y int32) {
	debug.Assert(lc != nil)
	debug.Assert(lastErr == nil)

	lc.SendCCmdTransformPlayer(id, x, y)
}

// NOTE(blukai): this is some hackery for GoSlice, idk yet
type Player = *protocol.NetworkedTransformPlayer

//export GetPlayers
func GetPlayers() []Player {
	debug.Assert(lc != nil)
	debug.Assert(lastErr == nil)

	return lc.GetPlayers()
}

// TODO: how can mod issue a disconnect when game is finished, etc?

func main() {
	// Connect(C.CString("udp4"), C.CString("127.0.0.1:8008"))
	// fmt.Println(lastErr)
}
