package fuse

import (
	"fmt"

	"github.com/jacobsa/fuse/fuseops"
)

func MapFlockType(t uint32) fuseops.FileLockType {
	switch t {
	case 0:
		return fuseops.F_RDLOCK
	case 1:
		return fuseops.F_WRLOCK
	case 2:
		return fuseops.F_UNLOCK
	}
	panic("MapFLockType: unknown type " + fmt.Sprintf("%d", t) + "!")
}

func UnmapFlockType(t fuseops.FileLockType) uint32 {
	var ret uint32
	switch t {
	case fuseops.F_RDLOCK:
		ret = 0
	case fuseops.F_WRLOCK:
		ret = 1
	case fuseops.F_UNLOCK:
		ret = 2
	}
	return ret
}
