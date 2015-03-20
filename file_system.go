// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fuse

import (
	"fmt"
	"os"
	"time"

	"github.com/jacobsa/bazilfuse"

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
	// This method is called once when mounting the file system. It must succeed
	// in order for the mount to succeed.
	Init(
		ctx context.Context,
		req *InitRequest) (*InitResponse, error)

	///////////////////////////////////
	// Inodes
	///////////////////////////////////

	// Look up a child by name within a parent directory. The kernel calls this
	// when resolving user paths to dentry structs, which are then cached.
	LookUpInode(
		ctx context.Context,
		req *LookUpInodeRequest) (*LookUpInodeResponse, error)

	// Refresh the attributes for an inode whose ID was previously returned by
	// LookUpInode. The kernel calls this when the FUSE VFS layer's cache of
	// inode attributes is stale. This is controlled by the AttributesExpiration
	// field of responses to LookUp, etc.
	GetInodeAttributes(
		ctx context.Context,
		req *GetInodeAttributesRequest) (*GetInodeAttributesResponse, error)

	// Change attributes for an inode.
	//
	// The kernel calls this for obvious cases like chmod(2), and for less
	// obvious cases like ftrunctate(2).
	SetInodeAttributes(
		ctx context.Context,
		req *SetInodeAttributesRequest) (*SetInodeAttributesResponse, error)

	// Forget an inode ID previously issued (e.g. by LookUpInode or MkDir). The
	// kernel calls this when removing an inode from its internal caches.
	ForgetInode(
		ctx context.Context,
		req *ForgetInodeRequest) (*ForgetInodeResponse, error)

	///////////////////////////////////
	// Inode creation
	///////////////////////////////////

	// Create a directory inode as a child of an existing directory inode. The
	// kernel sends this in response to a mkdir(2) call.
	//
	// The kernel appears to verify the name doesn't already exist (mkdir calls
	// mkdirat calls user_path_create calls filename_create, which verifies:
	// http://goo.gl/FZpLu5). But volatile file systems and paranoid non-volatile
	// file systems should check for the reasons described below on CreateFile.
	MkDir(
		ctx context.Context,
		req *MkDirRequest) (*MkDirResponse, error)

	// Create a file inode and open it.
	//
	// The kernel calls this method when the user asks to open a file with the
	// O_CREAT flag and the kernel has observed that the file doesn't exist. (See
	// for example lookup_open, http://goo.gl/PlqE9d).
	//
	// However it's impossible to tell for sure that all kernels make this check
	// in all cases and the official fuse documentation is less than encouraging
	// (" the file does not exist, first create it with the specified mode, and
	// then open it"). Therefore file systems would be smart to be paranoid and
	// check themselves, returning EEXIST when the file already exists. This of
	// course particularly applies to file systems that are volatile from the
	// kernel's point of view.
	CreateFile(
		ctx context.Context,
		req *CreateFileRequest) (*CreateFileResponse, error)

	///////////////////////////////////
	// Inode destruction
	///////////////////////////////////

	// Unlink a directory from its parent. Because directories cannot have a link
	// count above one, this means the directory inode should be deleted as well
	// once the kernel calls ForgetInode.
	//
	// The file system is responsible for checking that the directory is empty.
	//
	// Sample implementation in ext2: ext2_rmdir (http://goo.gl/B9QmFf)
	RmDir(
		ctx context.Context,
		req *RmDirRequest) (*RmDirResponse, error)

	// Unlink a file from its parent. If this brings the inode's link count to
	// zero, the inode should be deleted once the kernel calls ForgetInode. It
	// may still be referenced before then if a user still has the file open.
	//
	// Sample implementation in ext2: ext2_unlink (http://goo.gl/hY6r6C)
	Unlink(
		ctx context.Context,
		req *UnlinkRequest) (*UnlinkResponse, error)

	///////////////////////////////////
	// Directory handles
	///////////////////////////////////

	// Open a directory inode.
	//
	// On Linux the kernel calls this method when setting up a struct file for a
	// particular inode with type directory, usually in response to an open(2)
	// call from a user-space process. On OS X it may not be called for every
	// open(2) (cf. https://github.com/osxfuse/osxfuse/issues/199).
	OpenDir(
		ctx context.Context,
		req *OpenDirRequest) (*OpenDirResponse, error)

	// Read entries from a directory previously opened with OpenDir.
	ReadDir(
		ctx context.Context,
		req *ReadDirRequest) (*ReadDirResponse, error)

	// Release a previously-minted directory handle. The kernel calls this when
	// there are no more references to an open directory: all file descriptors
	// are closed and all memory mappings are unmapped.
	//
	// The kernel guarantees that the handle ID will not be used in further calls
	// to the file system (unless it is reissued by the file system).
	ReleaseDirHandle(
		ctx context.Context,
		req *ReleaseDirHandleRequest) (*ReleaseDirHandleResponse, error)

	///////////////////////////////////
	// File handles
	///////////////////////////////////

	// Open a file inode.
	//
	// On Linux the kernel calls this method when setting up a struct file for a
	// particular inode with type file, usually in response to an open(2) call
	// from a user-space process. On OS X it may not be called for every open(2)
	// (cf.https://github.com/osxfuse/osxfuse/issues/199).
	OpenFile(
		ctx context.Context,
		req *OpenFileRequest) (*OpenFileResponse, error)

	// Read data from a file previously opened with CreateFile or OpenFile.
	//
	// Note that this method is not called for every call to read(2) by the end
	// user; some reads may be served by the page cache. See notes on Write for
	// more.
	ReadFile(
		ctx context.Context,
		req *ReadFileRequest) (*ReadFileResponse, error)

	// Write data to a file previously opened with CreateFile or OpenFile.
	//
	// When the user writes data using write(2), the write goes into the page
	// cache and the page is marked dirty. Later the kernel may write back the
	// page via the FUSE VFS layer, causing this method to be called:
	//
	//  *  The kernel calls address_space_operations::writepage when a dirty page
	//     needs to be written to backing store (cf. http://goo.gl/Ezbewg). Fuse
	//     sets this to fuse_writepage (cf. http://goo.gl/IeNvLT).
	//
	//  *  (http://goo.gl/Eestuy) fuse_writepage calls fuse_writepage_locked.
	//
	//  *  (http://goo.gl/RqYIxY) fuse_writepage_locked makes a write request to
	//     the userspace server.
	//
	// Note that writes *will* be received before a call to Flush when closing
	// the file descriptor to which they were written:
	//
	//  *  (http://goo.gl/PheZjf) fuse_flush calls write_inode_now, which appears
	//     to start a writeback in the background (it talks about a "flusher
	//     thread").
	//
	//  *  (http://goo.gl/1IiepM) fuse_flush then calls fuse_sync_writes, which
	//     "[waits] for all pending writepages on the inode to finish".
	//
	//  *  (http://goo.gl/zzvxWv) Only then does fuse_flush finally send the
	//     flush request.
	//
	WriteFile(
		ctx context.Context,
		req *WriteFileRequest) (*WriteFileResponse, error)

	// Synchronize the current contents of an open file to storage.
	//
	// vfs.txt documents this as being called for by the fsync(2) system call
	// (cf. http://goo.gl/j9X8nB). Code walk for that case:
	//
	//  *  (http://goo.gl/IQkWZa) sys_fsync calls do_fsync, calls vfs_fsync, calls
	//     vfs_fsync_range.
	//  *  (http://goo.gl/5L2SMy) vfs_fsync_range calls f_op->fsync.
	//
	// Note that this is also called by fdatasync(2) (cf. http://goo.gl/01R7rF).
	//
	// See also: FlushFile, which may perform a similar purpose when closing a
	// file (but which is not used in "real" file systems).
	SyncFile(
		ctx context.Context,
		req *SyncFileRequest) (*SyncFileResponse, error)

	// Flush the current state of an open file to storage upon closing a file
	// descriptor.
	//
	// vfs.txt documents this as being called for each close(2) system call (cf.
	// http://goo.gl/FSkbrq). Code walk for that case:
	//
	//  *  (http://goo.gl/e3lv0e) sys_close calls __close_fd, calls filp_close.
	//  *  (http://goo.gl/nI8fxD) filp_close calls f_op->flush (fuse_flush).
	//
	// But note that this is also called in other contexts where a file
	// descriptor is closed, such as dup2(2) (cf. http://goo.gl/NQDvFS). In the
	// case of close(2), a flush error is returned to the user. For dup2(2), it
	// is not.
	//
	// Because of cases like dup2(2), calls to FlushFile are not necessarily one
	// to one with calls to OpenFile. They should not be used for reference
	// counting, and the handle must remain valid even after the method is called
	// (use ReleaseFileHandle to dispose of it).
	//
	// Typical "real" file systems do not implement this, presumably relying on
	// the kernel to write out the page cache to the block device eventually.
	// They can get away with this because a later open(2) will see the same
	// data. A file system that writes to remote storage however probably wants
	// to at least schedule a real flush, and maybe do it immediately in order to
	// return any errors that occur.
	FlushFile(
		ctx context.Context,
		req *FlushFileRequest) (*FlushFileResponse, error)

	// Release a previously-minted file handle. The kernel calls this when there
	// are no more references to an open file: all file descriptors are closed
	// and all memory mappings are unmapped.
	//
	// The kernel guarantees that the handle ID will not be used in further calls
	// to the file system (unless it is reissued by the file system).
	ReleaseFileHandle(
		ctx context.Context,
		req *ReleaseFileHandleRequest) (*ReleaseFileHandleResponse, error)
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
const RootInodeID = 1

func init() {
	// Make sure the constant above is correct. We do this at runtime rather than
	// defining the constant in terms of bazilfuse.RootID for two reasons:
	//
	//  1. Users can more clearly see that the root ID is low and can therefore
	//     be used as e.g. an array index, with space reserved up to the root.
	//
	//  2. The constant can be untyped and can therefore more easily be used as
	//     an array index.
	//
	if RootInodeID != bazilfuse.RootID {
		panic(
			fmt.Sprintf(
				"Oops, RootInodeID is wrong: %v vs. %v",
				RootInodeID,
				bazilfuse.RootID))
	}
}

// Attributes for a file or directory inode. Corresponds to struct inode (cf.
// http://goo.gl/tvYyQt).
type InodeAttributes struct {
	Size uint64

	// The number of incoming hard links to this inode.
	Nlink uint64

	// The mode of the inode. This is exposed to the user in e.g. the result of
	// fstat(2).
	//
	// Note that in contrast to the defaults for FUSE, this package mounts file
	// systems in a manner such that the kernel checks inode permissions in the
	// standard posix way. This is implemented by setting the default_permissions
	// mount option (cf. http://goo.gl/1LxOop and http://goo.gl/1pTjuk).
	//
	// For example, in the case of mkdir:
	//
	//  *  (http://goo.gl/JkdxDI) sys_mkdirat calls inode_permission.
	//
	//  *  (...) inode_permission eventually calls do_inode_permission.
	//
	//  *  (http://goo.gl/aGCsmZ) calls i_op->permission, which is
	//     fuse_permission (cf. http://goo.gl/VZ9beH).
	//
	//  *  (http://goo.gl/5kqUKO) fuse_permission doesn't do anything at all for
	//     several code paths if FUSE_DEFAULT_PERMISSIONS is unset. In contrast,
	//     if that flag *is* set, then it calls generic_permission.
	//
	Mode os.FileMode

	// Time information. See `man 2 stat` for full details.
	Atime  time.Time // Time of last access
	Mtime  time.Time // Time of last modification
	Ctime  time.Time // Time of last modification to inode
	Crtime time.Time // Time of creation (OS X only)

	// Ownership information
	Uid uint32
	Gid uint32
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

// A header that is included with every request.
type RequestHeader struct {
	// Credentials information for the process making the request.
	Uid uint32
	Gid uint32
}

// Information about a child inode within its parent directory. Shared by the
// responses for LookUpInode, MkDir, CreateFile, etc. Consumed by the kernel in
// order to set up a dcache entry.
type ChildInodeEntry struct {
	// The ID of the child inode. The file system must ensure that the returned
	// inode ID remains valid until a later call to ForgetInode.
	Child InodeID

	// A generation number for this incarnation of the inode with the given ID.
	// See comments on type GenerationNumber for more.
	Generation GenerationNumber

	// Current attributes for the child inode.
	//
	// When creating a new inode, the file system is responsible for initializing
	// and recording (where supported) attributes like time information,
	// ownership information, etc.
	//
	// Ownership information in particular must be set to something reasonable or
	// by default root will own everything and unprivileged users won't be able
	// to do anything useful. In traditional file systems in the kernel, the
	// function inode_init_owner (http://goo.gl/5qavg8) contains the
	// standards-compliant logic for this.
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
	//
	// Beware: this value is ignored on OS X, where entry caching is disabled by
	// default. See notes on MountConfig.EnableVnodeCaching for more.
	EntryExpiration time.Time
}

////////////////////////////////////////////////////////////////////////
// Requests and responses
////////////////////////////////////////////////////////////////////////

type InitRequest struct {
	Header RequestHeader
}

type InitResponse struct {
}

type LookUpInodeRequest struct {
	Header RequestHeader

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
	Entry ChildInodeEntry
}

type GetInodeAttributesRequest struct {
	Header RequestHeader

	// The inode of interest.
	Inode InodeID
}

type GetInodeAttributesResponse struct {
	// Attributes for the inode, and the time at which they should expire. See
	// notes on ChildInodeEntry.AttributesExpiration for more.
	Attributes           InodeAttributes
	AttributesExpiration time.Time
}

type SetInodeAttributesRequest struct {
	Header RequestHeader

	// The inode of interest.
	Inode InodeID

	// The attributes to modify, or nil for attributes that don't need a change.
	Size  *uint64
	Mode  *os.FileMode
	Atime *time.Time
	Mtime *time.Time
}

type SetInodeAttributesResponse struct {
	// The new attributes for the inode, and the time at which they should
	// expire. See notes on ChildInodeEntry.AttributesExpiration for more.
	Attributes           InodeAttributes
	AttributesExpiration time.Time
}

type ForgetInodeRequest struct {
	Header RequestHeader

	// The inode to be forgotten. The kernel guarantees that the node ID will not
	// be used in further calls to the file system (unless it is reissued by the
	// file system).
	ID InodeID
}

type ForgetInodeResponse struct {
}

type MkDirRequest struct {
	Header RequestHeader

	// The ID of parent directory inode within which to create the child.
	Parent InodeID

	// The name of the child to create, and the mode with which to create it.
	Name string
	Mode os.FileMode
}

type MkDirResponse struct {
	// Information about the inode that was created.
	Entry ChildInodeEntry
}

type CreateFileRequest struct {
	Header RequestHeader

	// The ID of parent directory inode within which to create the child file.
	Parent InodeID

	// The name of the child to create, and the mode with which to create it.
	Name string
	Mode os.FileMode

	// Flags for the open operation.
	Flags bazilfuse.OpenFlags
}

type CreateFileResponse struct {
	// Information about the inode that was created.
	Entry ChildInodeEntry

	// An opaque ID that will be echoed in follow-up calls for this file using
	// the same struct file in the kernel. In practice this usually means
	// follow-up calls using the file descriptor returned by open(2).
	//
	// The handle may be supplied to the following methods:
	//
	//  *  ReadFile
	//  *  WriteFile
	//  *  ReleaseFileHandle
	//
	// The file system must ensure this ID remains valid until a later call to
	// ReleaseFileHandle.
	Handle HandleID
}

type RmDirRequest struct {
	Header RequestHeader

	// The ID of parent directory inode, and the name of the directory being
	// removed within it.
	Parent InodeID
	Name   string
}

type UnlinkResponse struct {
}

type UnlinkRequest struct {
	Header RequestHeader

	// The ID of parent directory inode, and the name of the file being removed
	// within it.
	Parent InodeID
	Name   string
}

type RmDirResponse struct {
}

type OpenDirRequest struct {
	Header RequestHeader

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
	//  *  ReleaseDirHandle
	//
	// The file system must ensure this ID remains valid until a later call to
	// ReleaseDirHandle.
	Handle HandleID
}

type ReadDirRequest struct {
	Header RequestHeader

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

	// The maximum number of bytes to return in ReadDirResponse.Data. A smaller
	// number is acceptable.
	Size int
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
	//
	// An empty buffer indicates the end of the directory has been reached.
	Data []byte
}

type ReleaseDirHandleRequest struct {
	Header RequestHeader

	// The handle ID to be released. The kernel guarantees that this ID will not
	// be used in further calls to the file system (unless it is reissued by the
	// file system).
	Handle HandleID
}

type ReleaseDirHandleResponse struct {
}

type OpenFileRequest struct {
	Header RequestHeader

	// The ID of the inode to be opened.
	Inode InodeID

	// Mode and options flags.
	Flags bazilfuse.OpenFlags
}

type OpenFileResponse struct {
	// An opaque ID that will be echoed in follow-up calls for this file using
	// the same struct file in the kernel. In practice this usually means
	// follow-up calls using the file descriptor returned by open(2).
	//
	// The handle may be supplied to the following methods:
	//
	//  *  ReadFile
	//  *  WriteFile
	//  *  ReleaseFileHandle
	//
	// The file system must ensure this ID remains valid until a later call to
	// ReleaseFileHandle.
	Handle HandleID
}

type ReadFileRequest struct {
	Header RequestHeader

	// The file inode that we are reading, and the handle previously returned by
	// CreateFile or OpenFile when opening that inode.
	Inode  InodeID
	Handle HandleID

	// The range of the file to read.
	//
	// The FUSE documentation requires that exactly the number of bytes be
	// returned, except in the case of EOF or error (http://goo.gl/ZgfBkF). This
	// appears to be because it uses file mmapping machinery
	// (http://goo.gl/SGxnaN) to read a page at a time. It appears to understand
	// where EOF is by checking the inode size (http://goo.gl/0BkqKD), returned
	// by a previous call to LookUpInode, GetInodeAttributes, etc.
	Offset int64
	Size   int
}

type ReadFileResponse struct {
	// The data read. If this is less than the requested size, it indicates EOF.
	// An error should not be returned in this case.
	Data []byte
}

type WriteFileRequest struct {
	Header RequestHeader

	// The file inode that we are modifying, and the handle previously returned
	// by CreateFile or OpenFile when opening that inode.
	Inode  InodeID
	Handle HandleID

	// The offset at which to write the data below.
	//
	// The man page for pwrite(2) implies that aside from changing the file
	// handle's offset, using pwrite is equivalent to using lseek(2) and then
	// write(2). The man page for lseek(2) says the following:
	//
	// "The lseek() function allows the file offset to be set beyond the end of
	// the file (but this does not change the size of the file). If data is later
	// written at this point, subsequent reads of the data in the gap (a "hole")
	// return null bytes (aq\0aq) until data is actually written into the gap."
	//
	// It is therefore reasonable to assume that the kernel is looking for
	// the following semantics:
	//
	// *   If the offset is less than or equal to the current size, extend the
	//     file as necessary to fit any data that goes past the end of the file.
	//
	// *   If the offset is greater than the current size, extend the file
	//     with null bytes until it is not, then do the above.
	//
	Offset int64

	// The data to write.
	//
	// The FUSE documentation requires that exactly the number of bytes supplied
	// be written, except on error (http://goo.gl/KUpwwn). This appears to be
	// because it uses file mmapping machinery (http://goo.gl/SGxnaN) to write a
	// page at a time.
	Data []byte
}

type WriteFileResponse struct {
}

type SyncFileRequest struct {
	Header RequestHeader

	// The file and handle being sync'd.
	Inode  InodeID
	Handle HandleID
}

type SyncFileResponse struct {
}

type FlushFileRequest struct {
	Header RequestHeader

	// The file and handle being flushed.
	Inode  InodeID
	Handle HandleID
}

type FlushFileResponse struct {
}

type ReleaseFileHandleRequest struct {
	Header RequestHeader

	// The handle ID to be released. The kernel guarantees that this ID will not
	// be used in further calls to the file system (unless it is reissued by the
	// file system).
	Handle HandleID
}

type ReleaseFileHandleResponse struct {
}
