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
	CCmdPing
	CCmdJoin
	CCmdTransformPlayer

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
			body = ptr.To(NetworkedInt32(0))
		case CCmdTransformPlayer:
			body = ptr.To(NetworkedInt32Vector2{})
		// server
		case SCmdSetSeed:
			body = ptr.To(NetworkedInt32(0))
		case SCmdTransformPlayer:
			body = ptr.To(NetworkedPlayer{})
		}
		if body != nil {
			bodyBytes := data[CmdHeaderSize : CmdHeaderSize+cmd.Header.Size]
			err := cmd.Body.UnmarshalBinary(bodyBytes)
			if err != nil {
				return fmt.Errorf("could not unmarshal body: %w", err)
			}
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
	return byteorder.Htonl(zigzag.EncodeInt32(int32(*n))), nil
}

func (n *NetworkedInt32) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == 4)
	*n = NetworkedInt32(zigzag.ZigZagDecodeInt32(byteorder.Ntohl(data)))
	return nil
}

type NetworkedUint32 uint32

var (
	_ encoding.BinaryMarshaler   = (*NetworkedUint32)(nil)
	_ encoding.BinaryUnmarshaler = (*NetworkedUint32)(nil)
)

func (n *NetworkedUint32) MarshalBinary() ([]byte, error) {
	return byteorder.Htonl(uint32(*n)), nil
}

func (n *NetworkedUint32) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == 4)
	*n = NetworkedUint32(byteorder.Ntohl(data))
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

type NetworkedPlayer struct {
	// ID is uint32 representation of player's ip address, which is
	// completely garbage idea.
	//
	// TODO(blukai): figure out player identities
	ID        NetworkedUint32
	Transform NetworkedInt32Vector2
}

var (
	_ encoding.BinaryMarshaler   = (*NetworkedInt32Vector2)(nil)
	_ encoding.BinaryUnmarshaler = (*NetworkedInt32Vector2)(nil)
)

func (n *NetworkedPlayer) MarshalBinary() ([]byte, error) {
	buf := bytes.Buffer{}

	id, err := n.ID.MarshalBinary()
	debug.Assert(err == nil)
	buf.Write(id)

	transform, err := n.Transform.MarshalBinary()
	debug.Assert(err == nil)
	buf.Write(transform)

	return buf.Bytes(), nil
}

func (n *NetworkedPlayer) UnmarshalBinary(data []byte) error {
	debug.Assert(len(data) == 12)

	err := n.ID.UnmarshalBinary(data[0:4])
	debug.Assert(err == nil)

	err = n.Transform.UnmarshalBinary(data[4:12])
	debug.Assert(err == nil)

	return nil
}
