package fuse

import (
	"testing"
)

func BenchmarkGetPutInMessage(b *testing.B) {
	c := &Connection{}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			msg := c.getInMessage()
			c.putInMessage(msg)
		}
	})
}

func BenchmarkGetPutOutMessage(b *testing.B) {
	c := &Connection{}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			msg := c.getOutMessage()
			c.putOutMessage(msg)
		}
	})
}
