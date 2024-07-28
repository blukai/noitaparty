package byteorder

import (
	"encoding/binary"
)

// https://linux.die.net/man/3/ntohs
// https://github.com/vishvananda/netlink/blob/e5fd1f8193dee65ec93fafde8faf67e32a34692a/order.go

// decrypt names:
// h  = host
// n  = network
// s  = short     = 16 bit
// l  = long      = 32 bit
// ll = long long = 64 bit

func Htonl(val uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, val)
	return buf
}

func Htons(val uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, val)
	return buf
}

func Ntohl(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}

func Ntohs(buf []byte) uint16 {
	return binary.BigEndian.Uint16(buf)
}
