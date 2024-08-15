package lobbyserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/phuslu/log"
)

func makeAddrKey(addr *net.UDPAddr) uint64 {
	v4 := addr.IP.To4()
	// NOTE(blukai): atm i don't care about ipv6
	debug.Assert(v4 != nil)

	var result uint64

	result |= uint64(binary.BigEndian.Uint32(v4[:net.IPv4len]))
	// shift left to make room for port; port is uint16
	result <<= 16
	result |= uint64(addr.Port)

	return result
}

var idCounter = rand.Uint64()

// idea is stolen from
// https://github.com/rs/xid/blob/9d8d29f190786964cf2722e8e4d5c28c754b79ba/id.go#L144
//
// generateID generates a unique xid-like id, but with no machine and no pid,
// only timestamp and counter.
func generateID() uint64 {
	timestamp := uint64(time.Now().Unix())
	// TODO: can this counter overflow? if so what will happen then?
	randcount := atomic.AddUint64(&idCounter, 1)
	// last 4 bytes of timestamp and last 4 bytes of randcount
	return (timestamp << 32) | (randcount & ((1 << 32) - 1))
}

type client struct {
	addr     *net.UDPAddr
	lastSeen time.Time
	id       uint64
	name     string
}

type LobbyServer struct {
	worldSeed int32
	logger    *log.Logger

	conn    *net.UDPConn
	recvBuf []byte

	clients            map[uint64]*client
	evictionCandidates map[uint64]*client
	clientsMutex       *sync.RWMutex
}

func NewLobbyServer(
	network, address string,
	worldSeed int32,
	logger *log.Logger,
) (*LobbyServer, error) {
	// if logger is nil (which might be true in tests) => use default, but
	// silenced logger
	if logger == nil {
		tmp := log.DefaultLogger
		logger = &tmp
		logger.Writer = &log.IOWriter{Writer: io.Discard}
	}

	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, fmt.Errorf("could not resolve udp addr: %w", err)
	}
	conn, err := net.ListenUDP(network, addr)
	if err != nil {
		return nil, fmt.Errorf("could not listen udp: %w", err)
	}

	ls := &LobbyServer{
		worldSeed: worldSeed,
		logger:    logger,

		conn:    conn,
		recvBuf: make([]byte, protocol.MaxMsgSize),

		clients:            make(map[uint64]*client),
		evictionCandidates: make(map[uint64]*client),
		clientsMutex:       new(sync.RWMutex),
	}

	return ls, nil
}

// Addr can be useful to retreive server's address when LobbyServer was
// constructed with ":0".
func (ls *LobbyServer) Addr() *net.UDPAddr {
	return ls.conn.LocalAddr().(*net.UDPAddr)
}

func (ls *LobbyServer) encodeMsg(msgType protocol.MsgType, msg any) ([]byte, error) {
	// TODO: buffer pool or something
	w := &bytes.Buffer{}

	var b [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(b[:], uint64(msgType))
	if _, err := w.Write(b[:n]); err != nil {
		return nil, fmt.Errorf("could not write msg type: %w", err)
	}

	// NOTE: some msgs don't need "body"
	if msg != nil {
		if err := json.NewEncoder(w).Encode(msg); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

func (ls *LobbyServer) sendBytesTo(bytes []byte, addr *net.UDPAddr) error {
	// TODO: set write deadline?
	_, err := ls.conn.WriteToUDP(bytes, addr)
	return err
}

// msg must be a reference or nil
func (ls *LobbyServer) encodeAndSendMsgTo(
	msgType protocol.MsgType,
	msg any,
	addr *net.UDPAddr,
) error {
	ls.logger.Debug().
		Any("addr", addr).
		Uint32("msgType", msgType).
		Msg("encode and send msg")

	bytes, err := ls.encodeMsg(msgType, msg)
	if err != nil {
		return fmt.Errorf("could not encode msg: %w", err)
	}

	if err := ls.sendBytesTo(bytes, addr); err != nil {
		return fmt.Errorf("could not send bytes: %w", err)
	}

	return nil
}

func (ls *LobbyServer) encodeAndBroadcastMsg(
	msgType protocol.MsgType,
	msg any,
	srcAddr *net.UDPAddr,
) error {
	ls.logger.Debug().
		Any("srcAddr", srcAddr).
		Uint32("msgType", msgType).
		Msg("encode and broadcast msg")

	msgBytes, err := ls.encodeMsg(msgType, msg)
	if err != nil {
		return fmt.Errorf("could not encode msg: %v", err)
	}

	srcAddrKey := makeAddrKey(srcAddr)

	ls.clientsMutex.RLock()
	defer ls.clientsMutex.RUnlock()

	for addrKey, client := range ls.clients {
		// do not broadcast to the sender
		if addrKey == srcAddrKey {
			continue
		}

		// TODO: should this be in a go routine? if yes, make sure that
		// client.addr will remain "valid" within the go routine because
		// of go's look weirdness
		if err := ls.sendBytesTo(msgBytes, client.addr); err != nil {
			ls.logger.Error().
				Msgf("could not send bytes: %v", err)
			continue
		}
	}

	return nil
}

func (ls *LobbyServer) recvInner() error {
	err := ls.conn.SetReadDeadline(time.Now().Add(time.Second))
	if err != nil {
		return fmt.Errorf("could not set read deadline: %w", err)
	}

	n, addr, err := ls.conn.ReadFromUDP(ls.recvBuf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil
		}
		return fmt.Errorf("could not read from udp: %w", err)
	}

	buf := ls.recvBuf[:n]
	msgTypeUint64, msgTypeNumBytes := binary.Uvarint(buf)
	msgType := protocol.MsgType(msgTypeUint64)
	buf = buf[msgTypeNumBytes:]
	r := bytes.NewReader(buf)

	ls.logger.Debug().
		Any("addr", addr).
		Uint32("msgType", msgType).
		Msg("recv msg")

	switch msgType {
	case protocol.CHello:
		var msg protocol.CMsgHello
		if err := json.NewDecoder(r).Decode(&msg); err != nil {
			return fmt.Errorf("could not decode client hello: %w", err)
		}
		go ls.handleCHello(msg, addr)
	case protocol.APing:
		go ls.handleAPing(addr)
	case protocol.APong:
		// NOTE: pong does not need a response, but client's lastSeen
		// must be updated
	case protocol.ABroadcast:
		var msg protocol.AMsgBroadcast
		if err := json.NewDecoder(r).Decode(&msg); err != nil {
			return fmt.Errorf("could not decode client broadcast: %w", err)
		}
		go ls.handleAPlayerBroadcast(msg, addr)
	default:
		return fmt.Errorf("recv msg with invalid msg type: %d", msgType)
	}

	// NOTE: hello handler is responsible for client creation
	addrKey := makeAddrKey(addr)
	ls.clientsMutex.RLock()
	client, ok := ls.clients[addrKey]
	ls.clientsMutex.RUnlock()
	if ok {
		ls.clientsMutex.Lock()
		client.lastSeen = time.Now()
		delete(ls.evictionCandidates, addrKey)
		ls.clientsMutex.Unlock()
	}

	return nil
}

func (ls *LobbyServer) runRecv(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := ls.recvInner(); err != nil {
				ls.logger.Error().
					Msgf("recv err: %v", err)
			}
		}
	}
}

func (ls *LobbyServer) evictInner() {
	ls.clientsMutex.Lock()

	pingClients := make([]*client, 0)
	now := time.Now()
	for addrKey, client := range ls.clients {
		if now.Sub(client.lastSeen) < time.Second*5 {
			continue
		}

		if _, ok := ls.evictionCandidates[addrKey]; ok {
			continue
		}

		ls.evictionCandidates[addrKey] = client

		pingClients = append(pingClients, client)
	}

	disconnectClients := make([]*client, 0)
	for addrKey, client := range ls.evictionCandidates {
		if now.Sub(client.lastSeen) < time.Second*10 {
			continue
		}

		delete(ls.evictionCandidates, addrKey)
		delete(ls.clients, addrKey)

		disconnectClients = append(disconnectClients, client)
	}

	ls.clientsMutex.Unlock()

	for _, client := range pingClients {
		// TODO: should this be in a go routine? if yes, make sure that
		// client.addr will remain "valid" within the go routine because
		// of go's look weirdness
		if err := ls.encodeAndSendMsgTo(protocol.APing, nil, client.addr); err != nil {
			ls.logger.Error().
				Msgf("could not encode and send msg: %v", err)
		}
	}

	for _, client := range disconnectClients {
		sMsgPlayerDisconnected := protocol.SMsgPlayerDisconnected{
			ID: client.id,
		}
		err := ls.encodeAndBroadcastMsg(
			protocol.SPlayerDisconnected,
			&sMsgPlayerDisconnected,
			client.addr,
		)
		if err != nil {
			ls.logger.Error().
				Msgf("could not encode and broadcast player disconnected msg: %v", err)
		}
	}
}

func (ls *LobbyServer) runEvict(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
			ls.evictInner()
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
		ls.runEvict(ctx)
	}()

	select {
	case <-ctx.Done():
		wg.Wait()
		return ls.conn.Close()
	}
}

func (ls *LobbyServer) handleAPing(addr *net.UDPAddr) {
	if err := ls.encodeAndSendMsgTo(protocol.APong, nil, addr); err != nil {
		ls.logger.Error().
			Msgf("could not encode and send msg: %v", err)
	}
}

func (ls *LobbyServer) handleCHello(msg protocol.CMsgHello, addr *net.UDPAddr) {
	ls.clientsMutex.Lock()

	// NOTE: list of players that need to be sent within the welcome message
	// must not include the player itself.
	players := make([]protocol.SMsgWelcomePlayer, len(ls.clients))
	i := 0
	for _, client := range ls.clients {
		players[i] = protocol.SMsgWelcomePlayer{
			ID:   client.id,
			Name: client.name,
		}
		i += 1
	}

	addrKey := makeAddrKey(addr)
	if _, ok := ls.clients[addrKey]; !ok {
		ls.clients[addrKey] = &client{
			addr:     addr,
			lastSeen: time.Now(),
			id:       msg.PlayerID,
			name:     msg.PlayerName,
		}
	}

	ls.clientsMutex.Unlock()

	sMsgWelcome := protocol.SMsgWelcome{
		WorldSeed: ls.worldSeed,
		Players:   players,
	}
	err := ls.encodeAndSendMsgTo(protocol.SWelcome, &sMsgWelcome, addr)
	if err != nil {
		ls.logger.Error().
			Msgf("could not encode and send msg: %v", err)
		return
	}

	sMsgPlayerConnected := protocol.SMsgPlayerConnected{
		ID:   msg.PlayerID,
		Name: msg.PlayerName,
	}
	err = ls.encodeAndBroadcastMsg(protocol.SPlayerConnected, &sMsgPlayerConnected, addr)
	if err != nil {
		ls.logger.Error().
			Msgf("could not encode and broadcast player connected msg: %v", err)
		return
	}
}

func (ls *LobbyServer) handleAPlayerBroadcast(
	msg protocol.AMsgBroadcast,
	addr *net.UDPAddr,
) {
	if err := ls.encodeAndBroadcastMsg(protocol.ABroadcast, msg, addr); err != nil {
		ls.logger.Error().
			Msgf("could not encode and broadcast msg: %v", err)
	}
}
