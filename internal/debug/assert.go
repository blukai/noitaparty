package debug

import (
	"fmt"
	"runtime"
)

// NOTE: if you'll ever want to be able to turn off assertions, not remove, but
// turn off - take a look at
// https://sourcegraph.com/github.com/apache/arrow/-/blob/go/parquet/internal/debug/assert_off.go

// NOTE: originally stolen from
// https://github.com/golang/go/blob/eaa7d9ff86b35c72cc35bd7c14b349fa414c392f/src/go/types/errors.go#L18
func Assert(truth bool, msg ...string) {
	// NOTE: in certain cases it feels unreasonable and redundant to specify msg
	if len(msg) > 1 {
		panic("invalid assert args")
	}
	if !truth {
		msg := fmt.Sprintf("assertion failed(%s)", msg)
		// include information about the assertion location. due to
		// panic recovery, this location is otherwise buried in the
		// middle of the panicking stack.
		if _, file, line, ok := runtime.Caller(1); ok {
			msg = fmt.Sprintf("%s:%d: %s", file, line, msg)
		}
		panic(msg)
	}
}
