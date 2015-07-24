package fuseshim

import (
	"unsafe"

	"github.com/jacobsa/fuse/internal/fusekernel"
)

// Buffer provides a mechanism for constructing a message from multiple
// segments.
type Buffer []byte

// alloc allocates size bytes and returns a pointer to the new
// segment.
func (w *Buffer) Alloc(size uintptr) unsafe.Pointer {
	s := int(size)
	if len(*w)+s > cap(*w) {
		old := *w
		*w = make([]byte, len(*w), 2*cap(*w)+s)
		copy(*w, old)
	}
	l := len(*w)
	*w = (*w)[:l+s]
	return unsafe.Pointer(&(*w)[l])
}

// reset clears out the contents of the buffer.
func (w *Buffer) reset() {
	for i := range (*w)[:cap(*w)] {
		(*w)[i] = 0
	}
	*w = (*w)[:0]
}

func NewBuffer(extra uintptr) (buf Buffer) {
	const hdrSize = unsafe.Sizeof(fusekernel.OutHeader{})
	buf = make(Buffer, hdrSize, hdrSize+extra)
	return
}
