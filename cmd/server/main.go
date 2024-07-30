package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/blukai/noitaparty/internal/lobbyserver"
	"github.com/kelseyhightower/envconfig"
	"github.com/phuslu/log"
)

type Config struct {
	LobbyServerAddr4 string `encvonfig:"LOBBY_SERVER_ADDR4" required:"true" default:"0.0.0.0:5000"`
}

func loadConfig() (*Config, error) {
	config := new(Config)
	if err := envconfig.Process("", config); err != nil {
		return nil, err
	}
	return config, nil
}

func configureLogger() *log.Logger {
	logger := log.DefaultLogger

	// https://github.com/phuslu/log?tab=readme-ov-file#pretty-console-writer
	logger.Caller = 1
	logger.TimeFormat = "15:04:05"
	logger.Writer = &log.ConsoleWriter{
		ColorOutput:    true,
		QuoteString:    true,
		EndWithMessage: true,
	}

	return &logger
}

func erringMain() error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("could not process config: %w", err)
	}

	logger := configureLogger()

	lobbyServer, err := lobbyserver.NewLobbyServer("udp4", config.LobbyServerAddr4, logger)
	if err != nil {
		return fmt.Errorf("could not construct lobby server: %w", err)
	}
	logger.Info().Msgf("started lobby server on %s", config.LobbyServerAddr4)

	wg := new(sync.WaitGroup)
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	var lobbyServerRunErr error
	go func() {
		defer wg.Done()
		lobbyServerRunErr = lobbyServer.Run(ctx)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-signalChan:
		logger.Info().Msgf("received %+v signal", sig)
	}

	cancel()
	wg.Wait()
	if lobbyServerRunErr != nil {
		return fmt.Errorf("lobby server run failed: %w", err)
	}

	return nil
}

func main() {
	if err := erringMain(); err != nil {
		fmt.Fprintf(os.Stderr, "fucky wucky! %v\n", err)
		os.Exit(42)
	}
}
