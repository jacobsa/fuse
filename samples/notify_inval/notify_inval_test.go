package notify_inval_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/jacobsa/fuse/fusetesting"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/notify_inval"

	. "github.com/jacobsa/ogletest"
)

func TestNotifyInvalFS(t *testing.T) { RunTests(t) }

func (t *NotifyInvalFSTest) setTime(tv time.Time) {
	t.ticker.tickchan <- tv
	t.expectedTime = <-t.ticker.tockchan
}

func init() {
	RegisterTestSuite(&NotifyInvalFSTest{})
}

type manualTicker struct {
	tickchan chan time.Time
	tockchan chan time.Time
}

func (t *manualTicker) Ticks() <-chan time.Time { return t.tickchan }
func (t *manualTicker) Tocks() chan<- time.Time { return t.tockchan }

type NotifyInvalFSTest struct {
	samples.SampleTest

	ticker       *manualTicker
	expectedTime time.Time
}

func (t *NotifyInvalFSTest) SetUp(ti *TestInfo) {
	t.ticker = &manualTicker{
		tickchan: make(chan time.Time),
		tockchan: make(chan time.Time),
	}
	t.Server = notify_inval.NewNotifyInvalFS(t.ticker)
	t.SampleTest.SetUp(ti)
}

func (t *NotifyInvalFSTest) ReadDir_Root() {
	entries, err := fusetesting.ReadDirPicky(t.Dir)
	AssertEq(nil, err)
	AssertEq(2, len(entries))

	var fi os.FileInfo
	fi = entries[0]
	ExpectEq(t.expectedTime.Format(time.RFC3339), fi.Name())
	ExpectEq(0, fi.Size())
	ExpectEq(0444, fi.Mode())
	ExpectFalse(fi.IsDir())

	fi = entries[1]
	ExpectEq("current_time", fi.Name())
	ExpectEq(len(time.Time{}.Format(time.RFC3339))+1, fi.Size())
	ExpectEq(0444, fi.Mode())
	ExpectFalse(fi.IsDir())
}

func (t *NotifyInvalFSTest) ObserveTimeUpdate() {
	oldTime := t.expectedTime.Format(time.RFC3339)

	_, err := os.Stat(path.Join(t.Dir, oldTime))
	AssertEq(nil, err)
	slice, err := ioutil.ReadFile(path.Join(t.Dir, "current_time"))
	ExpectEq(oldTime+"\n", string(slice))

	t.setTime(t.expectedTime.Add(time.Minute))

	_, err = os.Stat(path.Join(t.Dir, oldTime))
	AssertNe(nil, err)
	slice, err = ioutil.ReadFile(path.Join(t.Dir, "current_time"))
	ExpectNe(oldTime+"\n", string(slice))
}
