package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples/notify_inval"
)

var mountPoint = flag.String("mountpoint", "", "directory to mount the filesystem")

type ticker struct {
	*time.Ticker
}

func (t *ticker) Ticks() <-chan time.Time {
	return t.Ticker.C
}

func (t *ticker) Tocks() chan<- time.Time { return nil }

func main() {
	flag.Parse()

	if *mountPoint == "" {
		log.Fatalf("--mountpoint is required")
	}

	t := &ticker{time.NewTicker(time.Second)}
	server := notify_inval.NewNotifyInvalFS(t)
	mfs, err := fuse.Mount(*mountPoint, server, &fuse.MountConfig{})
	if err != nil {
		panic(err)
	}
	if err := mfs.Join(context.Background()); err != nil {
		panic(err)
	}
}
