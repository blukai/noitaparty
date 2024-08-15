package protocol

type MsgType = uint32

const MaxMsgSize = 4 << 10

const (
	_ MsgType = iota

	// C = client
	CHello

	// S = server
	SWelcome
	SPlayerConnected
	SPlayerDisconnected

	// A = any (/ bidirectional)
	APing
	APong
	ABroadcast
)

type CMsgHello struct {
	// PlayerID is a 64-bit steam id.
	PlayerID uint64
	// PlayerName is steam persona name.
	PlayerName string
}

type SMsgWelcomePlayer struct {
	ID   uint64
	Name string
}

type SMsgWelcome struct {
	WorldSeed int32
	Players   []SMsgWelcomePlayer
}

type SMsgPlayerConnected struct {
	ID   uint64
	Name string
}

type SMsgPlayerDisconnected struct {
	ID uint64
}

type AMsgBroadcast struct {
	PlayerID uint64
	Data     []byte
}
