package memfs_test

import (
	"os"
	"path"
	"sync"
	"testing"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/memfs"
	. "github.com/jacobsa/ogletest"
)

func TestMessageProvider(t *testing.T) { RunTests(t) }

type MessageProviderTest struct {
	samples.SampleTest
	messageProviderTestImpl MessageProviderTestImpl
}

func init() { RegisterTestSuite(&MessageProviderTest{}) }

func (t *MessageProviderTest) SetUp(ti *TestInfo) {
	t.MountConfig.DisableWritebackCaching = true
	t.MountConfig.MessageProvider = &t.messageProviderTestImpl

	t.Server = memfs.NewMemFS(currentUid(), currentGid())
	t.SampleTest.SetUp(ti)
}

// The test suite is torn down during the test to ensure
// that all FUSE operations are complete before checking
// the invocations on the custom message provider.
func (t *MessageProviderTest) TearDown() {}

// A simple message provider class that wraps the default message provider
// and tracks how often each message is called.
type MessageProviderTestImpl struct {
	mu                     sync.RWMutex
	defaultMessageProvider fuse.DefaultMessageProvider
	getInMessageCount      int
	getOutMessageCount     int
	putInMessageCount      int
	putOutMessageCount     int
}

func (m *MessageProviderTestImpl) GetInMessage() *fuse.InMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getInMessageCount += 1
	return m.defaultMessageProvider.GetInMessage()
}

func (m *MessageProviderTestImpl) GetOutMessage() *fuse.OutMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getOutMessageCount += 1
	return m.defaultMessageProvider.GetOutMessage()
}

func (m *MessageProviderTestImpl) PutInMessage(x *fuse.InMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.putInMessageCount += 1
	m.defaultMessageProvider.PutInMessage(x)
}

func (m *MessageProviderTestImpl) PutOutMessage(x *fuse.OutMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.putOutMessageCount += 1
	m.defaultMessageProvider.PutOutMessage(x)
}

func (t *MessageProviderTest) TestCustomMessageProviderCallbacksInvoked() {
	var err error

	// Write a file.
	fileName := path.Join(t.Dir, memfs.CheckFileOpenFlagsFileName)
	const contents = "Hello world"
	err = os.WriteFile(fileName, []byte(contents), 0400)
	AssertEq(nil, err)

	// Read it back.
	slice, err := os.ReadFile(fileName)
	AssertEq(nil, err)
	ExpectEq(contents, string(slice))

	// Tear down the FUSE mount. This ensures that all FUSE operations are complete.
	t.SampleTest.TearDown()

	// Check that our custom message provider was invoked as expected.
	AssertGt(t.messageProviderTestImpl.getInMessageCount, 0)
	AssertGt(t.messageProviderTestImpl.getOutMessageCount, 0)
	AssertGt(t.messageProviderTestImpl.putInMessageCount, 0)
	AssertGt(t.messageProviderTestImpl.putOutMessageCount, 0)
	AssertEq(t.messageProviderTestImpl.getInMessageCount, t.messageProviderTestImpl.putInMessageCount)
	AssertEq(t.messageProviderTestImpl.getOutMessageCount, t.messageProviderTestImpl.putOutMessageCount)
}
