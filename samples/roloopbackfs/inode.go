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

package roloopbackfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

var (
	uid                     = uint32(os.Getuid())
	gid                     = uint32(os.Getgid())
	allocatedInodeId uint64 = fuseops.RootInodeID
)

func nextInodeID() (next fuseops.InodeID) {
	nextInodeId := atomic.AddUint64(&allocatedInodeId, 1)
	return fuseops.InodeID(nextInodeId)
}

type Inode interface {
	Id() fuseops.InodeID
	Path() string
	String() string
	Attributes() (*fuseops.InodeAttributes, error)
	ListChildren(inodes *sync.Map) ([]*fuseutil.Dirent, error)
	Contents() ([]byte, error)
}

func getOrCreateInode(inodes *sync.Map, parentId fuseops.InodeID, name string) (Inode, error) {
	parent, found := inodes.Load(parentId)
	if !found {
		return nil, nil
	}
	parentPath := parent.(Inode).Path()

	path := filepath.Join(parentPath, name)
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, nil
	}
	stat, _ := fileInfo.Sys().(*syscall.Stat_t)

	inodeEntry := &inodeEntry{
		id:   fuseops.InodeID(stat.Ino),
		path: path,
	}
	storedEntry, _ := inodes.LoadOrStore(inodeEntry.id, inodeEntry)
	return storedEntry.(Inode), nil
}

type inodeEntry struct {
	id   fuseops.InodeID
	path string
}

var _ Inode = &inodeEntry{}

func NewInode(path string) (Inode, error) {
	return &inodeEntry{
		id:   nextInodeID(),
		path: path,
	}, nil
}

func (in *inodeEntry) Id() fuseops.InodeID {
	return in.id
}

func (in *inodeEntry) Path() string {
	return in.path
}

func (in *inodeEntry) String() string {
	return fmt.Sprintf("%v::%v", in.id, in.path)
}

func (in *inodeEntry) Attributes() (*fuseops.InodeAttributes, error) {
	fileInfo, err := os.Stat(in.path)
	if err != nil {
		return &fuseops.InodeAttributes{}, err
	}

	return &fuseops.InodeAttributes{
		Size:  uint64(fileInfo.Size()),
		Nlink: 1,
		Mode:  fileInfo.Mode(),
		Mtime: fileInfo.ModTime(),
		Uid:   uid,
		Gid:   gid,
	}, nil
}

func (in *inodeEntry) ListChildren(inodes *sync.Map) ([]*fuseutil.Dirent, error) {
	children, err := ioutil.ReadDir(in.path)
	if err != nil {
		return nil, err
	}
	dirents := []*fuseutil.Dirent{}
	for i, child := range children {

		childInode, err := getOrCreateInode(inodes, in.id, child.Name())
		if err != nil || childInode == nil {
			continue
		}

		var childType fuseutil.DirentType
		if child.IsDir() {
			childType = fuseutil.DT_Directory
		} else if child.Mode()&os.ModeSymlink != 0 {
			childType = fuseutil.DT_Link
		} else {
			childType = fuseutil.DT_File
		}

		dirents = append(dirents, &fuseutil.Dirent{
			Offset: fuseops.DirOffset(i + 1),
			Inode:  childInode.Id(),
			Name:   child.Name(),
			Type:   childType,
		})
	}
	return dirents, nil
}

func (in *inodeEntry) Contents() ([]byte, error) {
	return ioutil.ReadFile(in.path)
}
