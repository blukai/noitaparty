package main

// #include <stdlib.h>
import "C"

import (
	"context"
	"os"
	"path/filepath"
	runtimedebug "runtime/debug"
	"time"
	"unsafe"

	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/lobbyclient"
	"github.com/blukai/noitaparty/internal/protocol"
)

var (
	lc      *lobbyclient.LobbyClient
	lastErr error
	cancel  context.CancelFunc
)

// mayneDumpStack is not absolutely panic-free, it theoretically may also panic
func maybeDumpStack() {
	if r := recover(); r == nil {
		return
	}

	// noita's root directory
	cwd, err := os.Getwd()
	debug.Assert(err == nil)

	filename := filepath.Join(
		cwd,
		"crashes",
		"noitaparty-"+time.Now().UTC().Format(time.RFC3339)+".txt",
	)
	stackTrace := runtimedebug.Stack()

	err = os.WriteFile(filename, stackTrace, 0644)
	debug.Assert(err == nil)

	panic("oopsie woopsie")
}

//export LastErr
func LastErr() *C.char {
	defer maybeDumpStack()

	if lastErr == nil {
		return nil
	}

	return C.CString(lastErr.Error())
}

//export Connect
func Connect(network, address *C.char) {
	defer maybeDumpStack()

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
	defer maybeDumpStack()

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
	defer maybeDumpStack()

	debug.Assert(lc != nil)
	debug.Assert(lastErr == nil)

	lc.SendCCmdTransformPlayer(id, x, y)
}

type CIter struct {
	len      int
	pos      int
	itemsPtr unsafe.Pointer
}

//export IterLen
func IterLen(iterPtr unsafe.Pointer) int {
	defer maybeDumpStack()

	iter := (*CIter)(iterPtr)
	return iter.len
}

//export IterPos
func IterPos(iterPtr unsafe.Pointer) int {
	defer maybeDumpStack()

	iter := (*CIter)(iterPtr)
	return iter.pos
}

//export IterHasNext
func IterHasNext(iterPtr unsafe.Pointer) bool {
	defer maybeDumpStack()

	iter := (*CIter)(iterPtr)
	return iter.len > iter.pos
}

//export IterFree
func IterFree(iterPtr unsafe.Pointer) {
	defer maybeDumpStack()

	C.free((*CIter)(iterPtr).itemsPtr)
	C.free(iterPtr)
}

//export GetNextPlayerInIter
func GetNextPlayerInIter(iterPtr unsafe.Pointer) unsafe.Pointer {
	defer maybeDumpStack()

	debug.Assert(iterPtr != nil)

	iter := (*CIter)(iterPtr)
	if iter.len > iter.pos {
		itemSize := unsafe.Sizeof(protocol.NetworkedTransformPlayer{})
		itemPtr := unsafe.Add(iter.itemsPtr, iter.pos*int(itemSize))

		iter.pos += 1

		return itemPtr
	}

	return nil
}

//export GetPlayerIter
func GetPlayerIter() unsafe.Pointer {
	defer maybeDumpStack()

	debug.Assert(lc != nil)
	debug.Assert(lastErr == nil)

	players := lc.GetPlayers()

	// NOTE(blukai): this is useful to fake player for local testing
	// players = []*protocol.NetworkedTransformPlayer{
	// 	{
	// 		ID: 42,
	// 		Transform: protocol.NetworkedInt32Vector2{
	// 			X: protocol.NetworkedInt32(rand.Intn(245-235+1) + 235),
	// 			Y: protocol.NetworkedInt32(rand.Int31n(275-265) - 256),
	// 		},
	// 	},
	// }

	// NOTE(blukai): it seems like the only way to return some non-owned
	// (possibly non-primitive) data to lua is to malloc memory and put the
	// value there

	itemSize := unsafe.Sizeof(protocol.NetworkedTransformPlayer{})

	itemsPtr := C.malloc(C.size_t(len(players) * int(itemSize)))
	for i, item := range players {
		itemPtr := unsafe.Add(itemsPtr, i*int(itemSize))
		*(*protocol.NetworkedTransformPlayer)(itemPtr) = *item
	}

	iterPtr := C.malloc(C.size_t(unsafe.Sizeof(CIter{})))
	*(*CIter)(iterPtr) = CIter{
		itemsPtr: itemsPtr,
		len:      len(players),
		pos:      0,
	}
	return iterPtr
}

// TODO: how can mod issue a disconnect when game is finished, etc?

func main() {
	// Connect(C.CString("udp4"), C.CString("127.0.0.1:5000"))
	// fmt.Println(lastErr)
}
