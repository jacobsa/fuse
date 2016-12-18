package buffer

import (
	"fmt"
	"testing"
	"unsafe"
)

func BenchmarkOutMessageReset(b *testing.B) {
	// A single buffer, which should fit in some level of CPU cache.
	b.Run("Single buffer", func(b *testing.B) {
		b.SetBytes(int64(unsafe.Sizeof(OutMessage{})))

		var om OutMessage
		for i := 0; i < b.N; i++ {
			om.Reset()
		}
	})

	// Many megabytes worth of buffers, which should defeat the CPU cache.
	b.Run("Many buffers", func(b *testing.B) {
		b.SetBytes(int64(unsafe.Sizeof(OutMessage{})))

		// The number of messages; intentionally a power of two.
		const numMessages = 128

		var oms [numMessages]OutMessage
		if s := unsafe.Sizeof(oms); s < 128<<20 {
			panic(fmt.Sprintf("Array is too small; total size: %d", s))
		}

		for i := 0; i < b.N; i++ {
			oms[i%numMessages].Reset()
		}
	})
}
