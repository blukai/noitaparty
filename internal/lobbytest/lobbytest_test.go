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
	// https://github.com/phuslu/log?tab=readme-ov-file#pretty-console-writer
	logger.Caller = 1
	logger.TimeFormat = "15:04:05"
	logger.Writer = &log.ConsoleWriter{
		ColorOutput:    true,
		QuoteString:    true,
		EndWithMessage: true,
	}

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

	playerOneX := int32(24)
	playerOneY := int32(13)

	// setup player two

	playerTwoClient, err := lobbyclient.NewLobbyClient("udp4", ls.Addr().String(), logger)
	is.NoErr(err)
	go playerTwoClient.Run(ctx)

	playerTwoID := uint64(2)

	// join player one

	t.Log("join one")
	playerOneSeed, err := playerOneClient.SendCCmdJoinRecvSCmdSetSeed(playerOneID)
	is.NoErr(err)

	// join player two

	t.Log("join two")
	playerTwoSeed, err := playerTwoClient.SendCCmdJoinRecvSCmdSetSeed(playerTwoID)
	is.NoErr(err)

	is.Equal(playerOneSeed, playerTwoSeed)

	// transform player one

	t.Log("transform player one")
	playerOneClient.SendCCmdTransformPlayer(playerOneID, playerOneX, playerOneY)
	// NOTE(blukai): need to sleep for a bit because client's send/recv is "async"
	time.Sleep(time.Millisecond)

	players := playerTwoClient.GetPlayers()
	is.Equal(len(players), 1)
	is.Equal(int32(players[0].Transform.X), playerOneX)
	is.Equal(int32(players[0].Transform.Y), playerOneY)
}
