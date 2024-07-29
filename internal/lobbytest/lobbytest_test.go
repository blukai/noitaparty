package lobbytest_test

import (
	"context"
	"testing"
	"time"

	"github.com/blukai/noitaparty/internal/lobbyclient"
	"github.com/blukai/noitaparty/internal/lobbyserver"
	"github.com/matryer/is"
	"github.com/phuslu/log"
)

func TestTwoPlayers(t *testing.T) {
	is := is.New(t)
	logger := &log.DefaultLogger

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ls, err := lobbyserver.NewLobbyServer("udp4", ":0", logger)
	is.NoErr(err)
	go ls.Run(ctx)

	// setup player one

	playerOneClient, err := lobbyclient.NewLobbyClient("udp4", ls.Addr().String(), logger)
	is.NoErr(err)
	go playerOneClient.Run(ctx)

	playerOneID := uint64(1)
	playerOneSeed := int32(111)

	playerOneX := int32(24)
	playerOneY := int32(13)

	// setup player two

	playerTwoClient, err := lobbyclient.NewLobbyClient("udp4", ls.Addr().String(), logger)
	is.NoErr(err)
	go playerTwoClient.Run(ctx)

	playerTwoID := uint64(2)
	playerTwoSeed := int32(222)

	// join player one

	seed := playerOneClient.SendCCmdJoinRecvSCmdSetSeed(playerOneID, playerOneSeed)
	is.Equal(seed, playerOneSeed)

	// join player two

	seed = playerTwoClient.SendCCmdJoinRecvSCmdSetSeed(playerTwoID, playerTwoSeed)
	is.Equal(seed, playerOneSeed)

	// transform player one

	playerOneClient.SendSCmdTransformPlayer(playerOneID, playerOneX, playerOneY)
	// NOTE(blukai): need to sleep for a bit because client's send/recv is "async"
	time.Sleep(time.Millisecond)
	players := playerTwoClient.GetPlayers()
	is.Equal(len(players), 1)
	is.Equal(int32(players[0].Transform.X), playerOneX)
	is.Equal(int32(players[0].Transform.Y), playerOneY)
}
