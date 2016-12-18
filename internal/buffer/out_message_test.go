package buffer

import (
	"crypto/rand"
	"fmt"
	"io"
	"testing"
	"unsafe"
)

func randBytes(n int) (b []byte, err error) {
	b = make([]byte, n)
	_, err = io.ReadFull(rand.Reader, b)
	return
}

func TestMemclr(t *testing.T) {
	// All sizes up to 32 bytes.
	var sizes []int
	for i := 0; i <= 32; i++ {
		sizes = append(sizes, i)
	}

	// And a few hand-chosen sizes.
	sizes = append(sizes, []int{
		39, 41, 64, 127, 128, 129,
		1<<20 - 1,
		1 << 20,
		1<<20 + 1,
	}...)

	// For each size, fill a buffer with random bytes and then zero it.
	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			// Generate
			b, err := randBytes(size)
			if err != nil {
				t.Fatalf("randBytes: %v", err)
			}

			// Clear
			var p unsafe.Pointer
			if len(b) != 0 {
				p = unsafe.Pointer(&b[0])
			}

			memclr(p, uintptr(len(b)))

			// Check
			for i, x := range b {
				if x != 0 {
					t.Fatalf("non-zero byte %d at offset %d", x, i)
				}
			}
		})
	}
}

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
