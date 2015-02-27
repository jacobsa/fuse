// Copyright 2015 Google Inc. All Rights Reserved.
// Author: jacobsa@google.com (Aaron Jacobs)

package fuse

import (
	"time"

	bazilfuse "bazil.org/fuse"

	"golang.org/x/net/context"
)

// An interface that must be implemented by file systems to be mounted with
// FUSE. See also the comments on request and response structs.
//
// Not all methods need to have interesting implementations. Embed a field of
// type fuseutil.NotImplementedFileSystem to inherit defaults that return
// ENOSYS to the kernel.
//
// Must be safe for concurrent access via all methods.
type FileSystem interface {
	// Look up a child by name within a parent directory. The kernel calls this
	// when resolving user paths to dentry structs, which are then cached.
	LookUpInode(
		ctx context.Context,
		req *LookUpInodeRequest) (*LookUpInodeResponse, error)

	// Forget an inode ID previously issued (e.g. by LookUpInode). The kernel
	// calls this when removing an inode from its internal caches.
	ForgetInode(
		ctx context.Context,
		req *ForgetInodeRequest) (*ForgetInodeResponse, error)

	// Open a directory inode. The kernel calls this method when setting up a
	// struct file for a particular inode with type directory, usually in
	// response to an open(2) call from a user-space process.
	OpenDir(
		ctx context.Context,
		req *OpenDirRequest) (*OpenDirResponse, error)

	// XXX: Comments
	ReadDir(
		ctx context.Context,
		req *ReadDirRequest) (*ReadDirResponse, error)

	// Release a previously-minted handle. The kernel calls this when there are
	// no more references to an open file: all file descriptors are closed and
	// all memory mappings are unmapped.
	//
	// The kernel guarantees that the handle ID will not be used in further calls
	// to the file system (unless it is reissued by the file system).
	ReleaseHandle(
		ctx context.Context,
		req *ReleaseHandleRequest) (*ReleaseHandleResponse, error)
}

////////////////////////////////////////////////////////////////////////
// Simple types
////////////////////////////////////////////////////////////////////////

// A 64-bit number used to uniquely identify a file or directory in the file
// system. File systems may mint inode IDs with any value except for
// RootInodeID.
//
// This corresponds to struct inode::i_no in the VFS layer.
// (Cf. http://goo.gl/tvYyQt)
type InodeID uint64

// A distinguished inode ID that identifies the root of the file system, e.g.
// in a request to OpenDir or LookUpInode. Unlike all other inode IDs, which
// are minted by the file system, the FUSE VFS layer may send a request for
// this ID without the file system ever having referenced it in a previous
// response.
const RootInodeID InodeID = InodeID(bazilfuse.RootID)

// Attributes for a file or directory inode. Corresponds to struct inode (cf.
// http://goo.gl/tvYyQt).
type InodeAttributes struct {
	// The size of the file in bytes.
	Size uint64
}

// A generation number for an inode. Irrelevant for file systems that won't be
// exported over NFS. For those that will and that reuse inode IDs when they
// become free, the generation number must change when an ID is reused.
//
// This corresponds to struct inode::i_generation in the VFS layer.
// (Cf. http://goo.gl/tvYyQt)
//
// Some related reading:
//
//     http://fuse.sourceforge.net/doxygen/structfuse__entry__param.html
//     http://stackoverflow.com/q/11071996/1505451
//     http://goo.gl/CqvwyX
//     http://julipedia.meroh.net/2005/09/nfs-file-handles.html
//     http://goo.gl/wvo3MB
//
type GenerationNumber uint64

// An opaque 64-bit number used to identify a particular open handle to a file
// or directory.
//
// This corresponds to fuse_file_info::fh.
type HandleID uint64

// An offset into an open directory handle. This is opaque to FUSE, and can be
// used for whatever purpose the file system desires. See notes on
// ReadDirRequest.Offset for details.
type DirOffset uint64

////////////////////////////////////////////////////////////////////////
// Requests and responses
////////////////////////////////////////////////////////////////////////

type LookUpInodeRequest struct {
	// The ID of the directory inode to which the child belongs.
	Parent InodeID

	// The name of the child of interest, relative to the parent. For example, in
	// this directory structure:
	//
	//     foo/
	//         bar/
	//             baz
	//
	// the file system may receive a request to look up the child named "bar" for
	// the parent foo/.
	Name string
}

type LookUpInodeResponse struct {
	// The ID of the child inode. The file system must ensure that the returned
	// inode ID remains valid until a later call to ForgetInode.
	Child InodeID

	// A generation number for this incarnation of the inode with the given ID.
	// See comments on type GenerationNumber for more.
	Generation GenerationNumber

	// Current ttributes for the child inode.
	Attributes InodeAttributes

	// The FUSE VFS layer in the kernel maintains a cache of file attributes,
	// used whenever up to date information about size, mode, etc. is needed.
	//
	// For example, this is the abridged call chain for fstat(2):
	//
	//  *  (http://goo.gl/tKBH1p) fstat calls vfs_fstat.
	//  *  (http://goo.gl/3HeITq) vfs_fstat eventuall calls vfs_getattr_nosec.
	//  *  (http://goo.gl/DccFQr) vfs_getattr_nosec calls i_op->getattr.
	//  *  (http://goo.gl/dpKkst) fuse_getattr calls fuse_update_attributes.
	//  *  (http://goo.gl/yNlqPw) fuse_update_attributes uses the values in the
	//     struct inode if allowed, otherwise calling out to the user-space code.
	//
	// In addition to obvious cases like fstat, this is also used in more subtle
	// cases like updating size information before seeking (http://goo.gl/2nnMFa)
	// or reading (http://goo.gl/FQSWs8).
	//
	// Most 'real' file systems do not set inode_operations::getattr, and
	// therefore vfs_getattr_nosec calls generic_fillattr which simply grabs the
	// information from the inode struct. This makes sense because these file
	// systems cannot spontaneously change; all modifications go through the
	// kernel which can update the inode struct as appropriate.
	//
	// In contrast, a FUSE file system may have spontaneous changes, so it calls
	// out to user space to fetch attributes. However this is expensive, so the
	// FUSE layer in the kernel caches the attributes if requested.
	//
	// This field controls when the attributes returned in this response and
	// stashed in the struct inode should be re-queried. Leave at the zero value
	// to disable caching.
	//
	// More reading:
	//     http://stackoverflow.com/q/21540315/1505451
	AttributesExpiration time.Time

	// The time until which the kernel may maintain an entry for this name to
	// inode mapping in its dentry cache. After this time, it will revalidate the
	// dentry.
	//
	// As in the discussion of attribute caching above, unlike real file systems,
	// FUSE file systems may spontaneously change their name -> inode mapping.
	// Therefore the FUSE VFS layer uses dentry_operations::d_revalidate
	// (http://goo.gl/dVea0h) to intercept lookups and revalidate by calling the
	// user-space LookUpInode method. However the latter may be slow, so it
	// caches the entries until the time defined by this field.
	//
	// Example code walk:
	//
	//     * (http://goo.gl/M2G3tO) lookup_dcache calls d_revalidate if enabled.
	//     * (http://goo.gl/ef0Elu) fuse_dentry_revalidate just uses the dentry's
	//     inode if fuse_dentry_time(entry) hasn't passed. Otherwise it sends a
	//     lookup request.
	//
	// Leave at the zero value to disable caching.
	EntryExpiration time.Time
}

type ForgetInodeRequest struct {
	// The inode to be forgotten. The kernel guarantees that the node ID will not
	// be used in further calls to the file system (unless it is reissued by the
	// file system).
	ID InodeID
}

type ForgetInodeResponse struct {
}

type OpenDirRequest struct {
	// The ID of the inode to be opened.
	Inode InodeID

	// Mode and options flags.
	Flags bazilfuse.OpenFlags
}

type OpenDirResponse struct {
	// An opaque ID that will be echoed in follow-up calls for this directory
	// using the same struct file in the kernel. In practice this usually means
	// follow-up calls using the file descriptor returned by open(2).
	//
	// The handle may be supplied to the following methods:
	//
	//  *  ReadDir
	//  *  ReleaseHandle
	//
	// The file system must ensure this ID remains valid until a later call to
	// ReleaseHandle.
	Handle HandleID
}

type ReadDirRequest struct {
	// The directory inode that we are reading, and the handle previously
	// returned by OpenDir when opening that inode.
	Inode  InodeID
	Handle HandleID

	// The offset within the directory at which to read.
	//
	// Warning: this field is not necessarily a count of bytes. Its legal values
	// are defined by the results returned in ReadDirResponse. See the notes
	// below and the notes on that struct.
	//
	// In the Linux kernel this ultimately comes from file::f_pos, which starts
	// at zero and is set by llseek and by the final consumed result returned by
	// each call to ReadDir:
	//
	//  *  (http://goo.gl/2nWJPL) iterate_dir, which is called by getdents(2) and
	//     readdir(2), sets dir_context::pos to file::f_pos before calling
	//     f_op->iterate, and then does the opposite assignment afterward.
	//
	//  *  (http://goo.gl/rTQVSL) fuse_readdir, which implements iterate for fuse
	//     directories, passes dir_context::pos as the offset to fuse_read_fill,
	//     which passes it on to user-space. fuse_readdir later calls
	//     parse_dirfile with the same context.
	//
	//  *  (http://goo.gl/vU5ukv) For each returned result (except perhaps the
	//     last, which may be truncated by the page boundary), parse_dirfile
	//     updates dir_context::pos with fuse_dirent::off.
	//
	// It is affected by the Posix directory stream interfaces in the following
	// manner:
	//
	//  *  (http://goo.gl/fQhbyn, http://goo.gl/ns1kDF) opendir initially causes
	//     filepos to be set to zero.
	//
	//  *  (http://goo.gl/ezNKyR, http://goo.gl/xOmDv0) readdir allows the user
	//     to iterate through the directory one entry at a time. As each entry is
	//     consumed, its d_off field is stored in __dirstream::filepos.
	//
	//  *  (http://goo.gl/WEOXG8, http://goo.gl/rjSXl3) telldir allows the user
	//     to obtain the d_off field from the most recently returned entry.
	//
	//  *  (http://goo.gl/WG3nDZ, http://goo.gl/Lp0U6W) seekdir allows the user
	//     to seek backward to an offset previously returned by telldir. It
	//     stores the new offset in filepos, and calls llseek to update the
	//     kernel's struct file.
	//
	//  *  (http://goo.gl/gONQhz, http://goo.gl/VlrQkc) rewinddir allows the user
	//     to go back to the beginning of the directory, obtaining a fresh view.
	//     It updates filepos and calls llseek to update the kernel's struct
	//     file.
	//
	// Unfortunately, FUSE offers no way to intercept seeks
	// (http://goo.gl/H6gEXa), so there is no way to cause seekdir or rewinddir
	// to fail. Additionally, there is no way to distinguish an explicit
	// rewinddir followed by readdir from the initial readdir, or a rewinddir
	// from a seekdir to the value returned by telldir just after opendir.
	//
	// Luckily, Posix is vague about what the user will see if they seek
	// backwards, and requires the user not to seek to an old offset after a
	// rewind. The only requirement on freshness is that rewinddir results in
	// something that looks like a newly-opened directory. So FUSE file systems
	// may e.g. cache an entire fresh listing for each ReadDir with a zero
	// offset, and return array offsets into that cached listing.
	Offset DirOffset

	// The maximum number of bytes to return in ReadDirResponse.Data.
	Size uint64
}

type ReadDirResponse struct {
	// A buffer consisting of a sequence of FUSE directory entries in the format
	// generated by fuse_add_direntry (http://goo.gl/qCcHCV), which is consumed
	// by parse_dirfile (http://goo.gl/2WUmD2). Use fuseutil.AppendDirent to
	// generate this data.
	//
	// The buffer must not exceed the length specified in ReadDirRequest.Size. It
	// is okay for the final entry to be truncated; parse_dirfile copes with this
	// by ignoring the partial record.
	//
	// Each entry returned exposes a directory offset to the user that may later
	// show up in ReadDirRequest.Offset. See notes on that field for more
	// information.
	Data []byte
}

type ReleaseHandleRequest struct {
	// The handle ID to be released. The kernel guarantees that this ID will not
	// be used in further calls to the file system (unless it is reissued by the
	// file system).
	Handle HandleID
}

type ReleaseHandleResponse struct {
}
