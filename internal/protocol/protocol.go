package protocol

import (
	"bytes"
	"encoding"
	"fmt"

	"github.com/blukai/noitaparty/internal/byteorder"
	"github.com/blukai/noitaparty/internal/debug"
	"github.com/blukai/noitaparty/internal/zigzag"
)

// TODO(blukai): consider using varint (/leb128) encoding for some numbers.

const (
	CmdHeaderSize = 4       // uint16 (2) + uint16 (2) = 4
	CmdMaxSize    = 4 << 10 // 4 * 1024 = 4096 bytes (4 is just an arbitrary number here)
)

const (
	// NOTE(blukai): C stands for client
	CCmdPing uint16 = 1 << iota
	CCmdJoin
	CCmdTransformPlayer
)

const (
	// NOTE(blukai): S stands for server
	SCmdPong uint16 = 1 << iota
	SCmdSetSeed
	SCmdSpawnPlayer
	// TODO(blukai): maybe it would be better to send player transforms in
	// batches?
	SCmdTransformPlayer
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

	header := &CmdHeader{}
	err := header.UnmarshalBinary(data[0:CmdHeaderSize])
	if err != nil {
		return fmt.Errorf("could not unmarshal header: %w", err)
	}
	cmd.Header = header

	if len(data) > CmdHeaderSize {
		// TODO(blukai): determine type of body from header and construct it, then unmarshal

		// err := cmd.Body.UnmarshalBinary(data[CmdHeaderSize : CmdHeaderSize+cmd.Header.Size])
		// if err != nil {
		// 	return fmt.Errorf("could not unmarshal body: %w", err)
		// }
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

	n.X.UnmarshalBinary(data[0:4])
	n.Y.UnmarshalBinary(data[4:8])

	return nil
}
