package protocol_test

import (
	"math"
	"testing"

	"github.com/blukai/noitaparty/internal/protocol"
	"github.com/matryer/is"
)

func TestCmdHeaderEncoding(t *testing.T) {
	is := is.New(t)

	originalCmdHeader := protocol.CmdHeader{
		Cmd:  protocol.CCmdPing,
		Size: 42,
	}

	encodedCmdHeaderBytes, err := originalCmdHeader.MarshalBinary()
	is.NoErr(err)
	is.Equal(len(encodedCmdHeaderBytes), protocol.CmdHeaderSize)

	decodedCmdHeader := protocol.CmdHeader{}
	err = decodedCmdHeader.UnmarshalBinary(encodedCmdHeaderBytes)
	is.NoErr(err)
	is.Equal(originalCmdHeader, decodedCmdHeader)
}

func TestCmdEncoding(t *testing.T) {
	is := is.New(t)

	t.Run("no body", func(t *testing.T) {
		originalCmd := protocol.Cmd{
			Header: &protocol.CmdHeader{
				Cmd: 42,
			},
		}

		encodedCmdBytes, err := originalCmd.MarshalBinary()
		is.NoErr(err)
		is.Equal(len(encodedCmdBytes), protocol.CmdHeaderSize)

		decodedCmd := protocol.Cmd{}
		err = decodedCmd.UnmarshalBinary(encodedCmdBytes)
		is.NoErr(err)
		is.Equal(originalCmd, decodedCmd)
	})

	t.Run("with body", func(t *testing.T) {
		// TODO(blukai): mashal cmd with body
		t.Skip()
	})
}

func TestNetworkedInt32Encoding(t *testing.T) {
	is := is.New(t)

	testCases := []int32{0, 1, -1, 42, -42, math.MaxInt32, math.MinInt32}

	for _, tc := range testCases {
		original := protocol.NetworkedInt32(tc)

		encoded, err := original.MarshalBinary()
		is.NoErr(err)
		is.Equal(len(encoded), 4)

		var decoded protocol.NetworkedInt32
		err = decoded.UnmarshalBinary(encoded)
		is.NoErr(err)
		is.Equal(original, decoded)
	}
}

func TestNetworkedInt32Vector2Encoding(t *testing.T) {
	is := is.New(t)

	testCases := []struct {
		x, y int32
	}{
		{0, 0},
		{1, -1},
		{42, 24},
		{math.MaxInt32, math.MinInt32},
	}

	for _, tc := range testCases {
		original := protocol.NetworkedInt32Vector2{
			X: protocol.NetworkedInt32(tc.x),
			Y: protocol.NetworkedInt32(tc.y),
		}

		encoded, err := original.MarshalBinary()
		is.NoErr(err)
		is.Equal(len(encoded), 8)

		var decoded protocol.NetworkedInt32Vector2
		err = decoded.UnmarshalBinary(encoded)
		is.NoErr(err)
		is.Equal(original, decoded)
	}
}
