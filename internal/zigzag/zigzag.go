package zigzag

// NOTE(blukai): this is stolen from valve's tier1/bitbuf.h

// ZigZag Transform:  Encodes signed integers so that they can be
// effectively used with varint encoding.
//
// varint operates on unsigned integers, encoding smaller numbers into
// fewer bytes.  If you try to use it on a signed integer, it will treat
// this number as a very large unsigned integer, which means that even
// small signed numbers like -1 will take the maximum number of bytes
// (10) to encode.  ZigZagEncode() maps signed integers to unsigned
// in such a way that those with a small absolute value will have smaller
// encoded values, making them appropriate for encoding using varint.
//
//       int32 ->     uint32
// -------------------------
//           0 ->          0
//          -1 ->          1
//           1 ->          2
//          -2 ->          3
//         ... ->        ...
//  2147483647 -> 4294967294
// -2147483648 -> 4294967295
//
//        >> encode >>
//        << decode <<

func Encode32(n int32) uint32 {
	return uint32((n << 1) ^ (n >> 31))
}

func Decode32(n uint32) int32 {
	return int32(n>>1) ^ -int32(n&1)
}

func Encode64(n int64) uint64 {
	return uint64((n << 1) ^ (n >> 63))
}

func Decode64(n uint64) int64 {
	return int64(n>>1) ^ -int64(n&1)
}
