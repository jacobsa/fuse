package memfs_test

import (
	"os"
	"path"
	"testing"

	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/memfs"
	. "github.com/jacobsa/ogletest"
)

func TestFuseServerCallbacks(t *testing.T) { RunTests(t) }

type CallbackTest struct {
	samples.SampleTest
	readFileCallbackInvoked  bool
	writeFileCallbackInvoked bool
}

func init() { RegisterTestSuite(&CallbackTest{}) }

func (t *CallbackTest) SetUp(ti *TestInfo) {
	t.MountConfig.DisableWritebackCaching = true
	t.readFileCallbackInvoked = false
	t.writeFileCallbackInvoked = false

	t.Server = memfs.NewMemFSWithCallbacks(
		currentUid(),
		currentGid(),
		func() { t.readFileCallbackInvoked = true },
		func() { t.writeFileCallbackInvoked = true },
	)
	t.SampleTest.SetUp(ti)
}

// The test suite is torn down during the test to ensure
// that all FUSE operations are complete before checking
// the invocations on the callbacks.
func (t *CallbackTest) TearDown() {}

func (t *CallbackTest) TestCallbacksInvokedForWriteFile() {
	AssertEq(t.writeFileCallbackInvoked, false)
	AssertEq(t.readFileCallbackInvoked, false)

	// Write a file.
	fileName := path.Join(t.Dir, memfs.CheckFileOpenFlagsFileName)
	const contents = "Hello world"
	err := os.WriteFile(fileName, []byte(contents), 0400)
	AssertEq(nil, err)

	// Tear down the FUSE mount. This ensures that all FUSE operations are complete.
	t.SampleTest.TearDown()

	// Check that our callback was invoked as expected.
	AssertEq(t.writeFileCallbackInvoked, true)
	AssertEq(t.readFileCallbackInvoked, false)
}

func (t *CallbackTest) TestCallbacksInvokedForWriteFileAndReadFile() {
	AssertEq(t.writeFileCallbackInvoked, false)
	AssertEq(t.readFileCallbackInvoked, false)

	// Write a file.
	fileName := path.Join(t.Dir, memfs.CheckFileOpenFlagsFileName)
	const contents = "Hello world"
	err := os.WriteFile(fileName, []byte(contents), 0400)
	AssertEq(nil, err)

	// Read it back.
	slice, err := os.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq(contents, string(slice))

	// Tear down the FUSE mount. This ensures that all FUSE operations are complete.
	t.SampleTest.TearDown()

	// Check that our callback was invoked as expected.
	AssertEq(t.writeFileCallbackInvoked, true)
	AssertEq(t.readFileCallbackInvoked, true)
}
