package protocol

import (
	"bytes"
	"encoding"
	"fmt"

	"github.com/blukai/noitaparty/internal/byteorder"
	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/ptr"
	"github.com/blukai/noitaparty/internal/zigzag"
)

// TODO(blukai): consider using varint (/leb128) encoding for some numbers.

const (
	CmdHeaderSize = 4       // uint16 (2) + uint16 (2) = 4
	CmdMaxSize    = 4 << 10 // 4 * 1024 = 4096 bytes (4 is just an arbitrary number here)
)

const (
	// NOTE(blukai): C stands for client
	_ uint16 = iota
	// respond with SCmdPong
	CCmdPing
	// respond with SCmdSetSeed
	CCmdJoin
	// no response
	CCmdTransformPlayer
	// no response; if client stopped sending keep alive messages server
	// must assume that client is not connected anymore
	CCmdKeepAlive

	CCmdMax
)

const (
	// NOTE(blukai): S stands for server
	_ uint16 = iota + CCmdMax
	SCmdPong
	SCmdSetSeed
	// TODO(blukai): maybe it would be better to send player transforms in
	// batches?
	SCmdTransformPlayer
	// TODO(blukai): spawn player (on connect)
	// TODO(blukai): despawn player (on disconnect)

	SCmdMax
)

type CmdHeader struct {
	Cmd  uint16
	Size uint16
}

var (
	_ encoding.BinaryMarshaler   = (*CmdHeader)(nil)
	_ encoding.BinaryUnmarshaler = (*CmdHeader)(nil)
)

func (h *CmdHeader) MarshalBinary() ([]byte, error) {
	buf := bytes.Buffer{}

	buf.Write(byteorder.Htons(h.Cmd))
	buf.Write(byteorder.Htons(h.Size))

	data := buf.Bytes()
	debug.Assert(len(data) == CmdHeaderSize)

	return data, nil
}

func (h *CmdHeader) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == CmdHeaderSize)

	h.Cmd = byteorder.Ntohs(data[0:2])
	h.Size = byteorder.Ntohs(data[2:4])

	return nil
}

// TODO: get rid of CmdBody and define bodies for each cmd + command
// constructors
type CmdBody interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

type Cmd struct {
	Header *CmdHeader
	Body   CmdBody
}

var (
	_ encoding.BinaryMarshaler   = (*Cmd)(nil)
	_ encoding.BinaryUnmarshaler = (*Cmd)(nil)
)

func (cmd *Cmd) MarshalBinary() ([]byte, error) {
	buf := bytes.Buffer{}

	headerBytes, err := cmd.Header.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("could not marshal header: %w", err)
	}
	buf.Write(headerBytes)

	if cmd.Body != nil {
		bodyBytes, err := cmd.Body.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("could not marshal body: %w", err)
		}
		buf.Write(bodyBytes)
	}

	data := buf.Bytes()
	debug.Assert(len(data) >= CmdHeaderSize)

	return data, nil
}

func (cmd *Cmd) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) >= CmdHeaderSize)

	headerBytes := data[0:CmdHeaderSize]
	header := &CmdHeader{}
	err := header.UnmarshalBinary(headerBytes)
	if err != nil {
		return fmt.Errorf("could not unmarshal header: %w", err)
	}
	cmd.Header = header

	if len(data) > CmdHeaderSize {
		body := (CmdBody)(nil)
		switch cmd.Header.Cmd {
		// client
		case CCmdJoin:
			body = ptr.To(NetworkedUint64(0))
		case CCmdTransformPlayer:
			body = &NetworkedTransformPlayer{}
		// server
		case SCmdSetSeed:
			body = ptr.To(NetworkedInt32(0))
		case SCmdTransformPlayer:
			body = &NetworkedTransformPlayer{}
		}
		if body != nil {
			bodyBytes := data[CmdHeaderSize : CmdHeaderSize+cmd.Header.Size]
			err := body.UnmarshalBinary(bodyBytes)
			if err != nil {
				return fmt.Errorf("could not unmarshal body: %w", err)
			}
			cmd.Body = body
		}
	}

	return nil
}

type NetworkedInt32 int32

var (
	_ encoding.BinaryMarshaler   = (*NetworkedInt32)(nil)
	_ encoding.BinaryUnmarshaler = (*NetworkedInt32)(nil)
)

func (n *NetworkedInt32) MarshalBinary() ([]byte, error) {
	return byteorder.Htonl(zigzag.Encode32(int32(*n))), nil
}

func (n *NetworkedInt32) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == 4)
	*n = NetworkedInt32(zigzag.Decode32(byteorder.Ntohl(data)))
	return nil
}

type NetworkedUint64 uint64

var (
	_ encoding.BinaryMarshaler   = (*NetworkedUint64)(nil)
	_ encoding.BinaryUnmarshaler = (*NetworkedUint64)(nil)
)

func (n *NetworkedUint64) MarshalBinary() ([]byte, error) {
	return byteorder.Htonll(uint64(*n)), nil
}

func (n *NetworkedUint64) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == 8)
	*n = NetworkedUint64(byteorder.Ntohll(data))
	return nil
}

type NetworkedInt32Vector2 struct {
	X NetworkedInt32
	Y NetworkedInt32
}

var (
	_ encoding.BinaryMarshaler   = (*NetworkedInt32Vector2)(nil)
	_ encoding.BinaryUnmarshaler = (*NetworkedInt32Vector2)(nil)
)

func (n *NetworkedInt32Vector2) MarshalBinary() ([]byte, error) {
	buf := bytes.Buffer{}

	x, err := n.X.MarshalBinary()
	debug.Assert(err == nil)
	buf.Write(x)

	y, err := n.Y.MarshalBinary()
	debug.Assert(err == nil)
	buf.Write(y)

	return buf.Bytes(), nil
}

func (n *NetworkedInt32Vector2) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == 8)

	err := n.X.UnmarshalBinary(data[0:4])
	debug.Assert(err == nil)

	err = n.Y.UnmarshalBinary(data[4:8])
	debug.Assert(err == nil)

	return nil
}

type NetworkedTransformPlayer struct {
	ID        NetworkedUint64
	Transform NetworkedInt32Vector2
}

var (
	_ encoding.BinaryMarshaler   = (*NetworkedInt32Vector2)(nil)
	_ encoding.BinaryUnmarshaler = (*NetworkedInt32Vector2)(nil)
)

func (n *NetworkedTransformPlayer) MarshalBinary() ([]byte, error) {
	buf := bytes.Buffer{}

	id, err := n.ID.MarshalBinary()
	debug.Assert(err == nil)
	buf.Write(id)

	transform, err := n.Transform.MarshalBinary()
	debug.Assert(err == nil)
	buf.Write(transform)

	return buf.Bytes(), nil
}

func (n *NetworkedTransformPlayer) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == 16)

	err := n.ID.UnmarshalBinary(data[0:8])
	debug.Assert(err == nil)

	err = n.Transform.UnmarshalBinary(data[8:16])
	debug.Assert(err == nil)

	return nil
}
