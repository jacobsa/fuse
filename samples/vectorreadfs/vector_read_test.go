package vector_read_fs_test

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/fuse/samples"
	. "github.com/jacobsa/ogletest"
)

func TestVectorRead(t *testing.T) { RunTests(t) }

type VectorReadTest struct {
	samples.SampleTest
	vectorReadFs *vectorReadFs
	testInfo     *TestInfo
}

func init() { RegisterTestSuite(&VectorReadTest{}) }

const testFileSize = 1024
const testFileName = "testFileName"

// A simple file system implementation that only exposes the
// existence of a single test file for reading and allows
// tests to implement custom read functions.
type vectorReadFs struct {
	fuseutil.NotImplementedFileSystem
	readFileFn func(ctx context.Context, op *fuseops.ReadFileOp) error
}

func (fs *vectorReadFs) StatFS(ctx context.Context, op *fuseops.StatFSOp) error {
	return nil
}

func (fs *vectorReadFs) LookUpInode(ctx context.Context, op *fuseops.LookUpInodeOp) error {
	if op.Name == testFileName {
		op.Entry = fuseops.ChildInodeEntry{
			Child: 2,
			Attributes: fuseops.InodeAttributes{
				Size:  testFileSize,
				Nlink: 1,
				Mode:  0444,
			},
		}
		return nil
	}
	return fuse.ENOENT
}

func (fs *vectorReadFs) GetInodeAttributes(ctx context.Context, op *fuseops.GetInodeAttributesOp) error {
	if op.Inode == 1 {
		op.Attributes = fuseops.InodeAttributes{
			Nlink: 1,
			Mode:  0755 | os.ModeDir,
		}
		return nil
	} else if op.Inode == 2 {
		op.Attributes = fuseops.InodeAttributes{
			Size:  testFileSize,
			Nlink: 1,
			Mode:  0444,
		}
		return nil
	}
	return fuse.ENOENT
}

func (fs *vectorReadFs) OpenDir(ctx context.Context, op *fuseops.OpenDirOp) error {
	return nil
}

func (fs *vectorReadFs) ReadDir(ctx context.Context, op *fuseops.ReadDirOp) error {
	return nil
}

func (fs *vectorReadFs) OpenFile(ctx context.Context, op *fuseops.OpenFileOp) error {
	return nil
}

func (fs *vectorReadFs) ReadFile(ctx context.Context, op *fuseops.ReadFileOp) error {
	if fs.readFileFn != nil {
		return fs.readFileFn(ctx, op)
	}
	return nil
}

func (fs *vectorReadFs) ReleaseDirHandle(ctx context.Context, op *fuseops.ReleaseDirHandleOp) error {
	return nil
}

func (fs *vectorReadFs) GetXattr(ctx context.Context, op *fuseops.GetXattrOp) error {
	return nil
}

func (fs *vectorReadFs) ListXattr(ctx context.Context, op *fuseops.ListXattrOp) error {
	return nil
}

func (fs *vectorReadFs) ForgetInode(ctx context.Context, op *fuseops.ForgetInodeOp) error {
	return nil
}

func (fs *vectorReadFs) ReleaseFileHandle(ctx context.Context, op *fuseops.ReleaseFileHandleOp) error {
	return nil
}

func (fs *vectorReadFs) FlushFile(ctx context.Context, op *fuseops.FlushFileOp) error {
	return nil
}

func (t *VectorReadTest) SetUp(ti *TestInfo) {
	t.vectorReadFs = &vectorReadFs{}
	t.Server = fuseutil.NewFileSystemServer(t.vectorReadFs)
	t.testInfo = ti
}

func (t *VectorReadTest) TestReadBufferAllocated() {
	// Set up the fuse server config to allocate read buffers
	t.MountConfig.UseVectoredRead = true
	t.MountConfig.AllocateReadBufferForVectoredRead = true
	t.SampleTest.SetUp(t.testInfo)

	// Use a custom read function that verifies the presence of the pre-allocated Dst buffer
	t.vectorReadFs.readFileFn = func(ctx context.Context, op *fuseops.ReadFileOp) error {
		if op.Dst == nil {
			return fuse.EIO
		}
		if int64(len(op.Dst)) != op.Size {
			return fuse.EINVAL
		}

		op.BytesRead = int(op.Size)
		if testFileSize-int(op.Offset) < int(op.Size) {
			op.BytesRead = testFileSize - int(op.Offset)
		}

		t.generateFileContent(op.Dst[:op.BytesRead])
		op.Data = append(op.Data, op.Dst[:op.BytesRead])
		return nil
	}

	// Read the test file and verify the output
	fileName := path.Join(t.Dir, testFileName)
	fileContent, err := os.ReadFile(fileName)
	AssertEq(nil, err)
	AssertEq(testFileSize, len(fileContent))

	expectedContent := make([]byte, testFileSize)
	t.generateFileContent(expectedContent)
	ExpectEq(string(expectedContent), string(fileContent))
}

func (t *VectorReadTest) TestReadBufferNotAllocated() {
	// Set up the fuse server config to use vectored reads but not to allocate read buffers
	t.MountConfig.UseVectoredRead = true
	t.MountConfig.AllocateReadBufferForVectoredRead = false
	t.SampleTest.SetUp(t.testInfo)

	// Use a custom read function that verifies that the Dst buffer is not pre-allocated
	t.vectorReadFs.readFileFn = func(ctx context.Context, op *fuseops.ReadFileOp) error {
		if op.Dst != nil {
			return fuse.EIO
		}

		op.BytesRead = int(op.Size)
		if testFileSize-int(op.Offset) < int(op.Size) {
			op.BytesRead = testFileSize - int(op.Offset)
		}

		buf := make([]byte, op.BytesRead)
		t.generateFileContent(buf)
		op.Data = append(op.Data, buf)
		return nil
	}

	// Read the test file and verify the output
	fileName := path.Join(t.Dir, testFileName)
	fileContent, err := os.ReadFile(fileName)
	AssertEq(nil, err)
	AssertEq(testFileSize, len(fileContent))

	expectedContent := make([]byte, testFileSize)
	t.generateFileContent(expectedContent)
	ExpectEq(string(expectedContent), string(fileContent))
}

// Fills the input buffer with sequentially increasing bytes. The sequence length
// is prime to help detect duplication errors.
func (t *VectorReadTest) generateFileContent(buf []byte) {
	for i := 0; i < len(buf); i++ {
		buf[i] = byte(i % 199)
	}
}
