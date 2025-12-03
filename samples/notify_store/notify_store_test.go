package notify_store_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/jacobsa/fuse/fusetesting"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/notify_store"
	. "github.com/jacobsa/ogletest"
)

func TestNotifyStoreFS(t *testing.T) { RunTests(t) }

func (t *NotifyStoreFSTest) setTime(tv time.Time) {
	t.ticker.tickchan <- tv
	t.expectedTime = <-t.ticker.tockchan
}

func init() {
	RegisterTestSuite(&NotifyStoreFSTest{})
}

type manualTicker struct {
	tickchan chan time.Time
	tockchan chan time.Time
}

func (t *manualTicker) Ticks() <-chan time.Time { return t.tickchan }
func (t *manualTicker) Tocks() chan<- time.Time { return t.tockchan }

type NotifyStoreFSTest struct {
	samples.SampleTest

	ticker       *manualTicker
	expectedTime time.Time
}

func (t *NotifyStoreFSTest) SetUp(ti *TestInfo) {
	t.ticker = &manualTicker{
		tickchan: make(chan time.Time),
		tockchan: make(chan time.Time),
	}
	t.Server = notify_store.NewNotifyStoreFS(t.ticker)
	t.SampleTest.SetUp(ti)
}

func (t *NotifyStoreFSTest) ReadDir_Root() {
	entries, err := fusetesting.ReadDirPicky(t.Dir)
	AssertEq(nil, err)
	AssertEq(1, len(entries))

	var fi os.FileInfo
	fi = entries[0]
	ExpectEq("current_time", fi.Name())
	ExpectEq(len(time.Time{}.Format(time.RFC3339))+1, fi.Size())
	ExpectEq(0444, fi.Mode())
	ExpectFalse(fi.IsDir())
}

func (t *NotifyStoreFSTest) ObserveTimeUpdate() {
	oldTime := t.expectedTime.Format(time.RFC3339)

	slice, err := ioutil.ReadFile(path.Join(t.Dir, "current_time"))
	ExpectEq(err, nil)
	ExpectEq(oldTime+"\n", string(slice))

	t.setTime(t.expectedTime.Add(time.Minute))

	slice, err = ioutil.ReadFile(path.Join(t.Dir, "current_time"))
	ExpectEq(err, nil)
	ExpectNe(oldTime+"\n", string(slice))
}
