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

package roloopbackfs_test

import (
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacobsa/fuse/samples"
	"github.com/jacobsa/fuse/samples/roloopbackfs"
	. "github.com/jacobsa/ogletest"
	"io/ioutil"
	"os"
)

var (
	letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func TestReadonlyLoopbackFS(t *testing.T) { RunTests(t) }

type ReadonlyLoopbackFSTest struct {
	samples.SampleTest
	physicalPath string
}

func init() {
	RegisterTestSuite(&ReadonlyLoopbackFSTest{})
	rand.Seed(time.Now().UnixNano())
}

func (t *ReadonlyLoopbackFSTest) SetUp(ti *TestInfo) {
	var err error

	t.physicalPath, err = ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(t.physicalPath, 0777)
	if err != nil {
		panic(err)
	}

	t.fillPhysicalFS()

	t.Server, err = roloopbackfs.NewReadonlyLoopbackServer(
		t.physicalPath,
		log.New(os.Stdout, "", 0),
	)
	AssertEq(nil, err)
	t.SampleTest.SetUp(ti)
}

func (t *ReadonlyLoopbackFSTest) TearDown() {
	t.SampleTest.TearDown()
	err := os.RemoveAll(t.physicalPath)
	if err != nil {
		panic(err)
	}
}

func createDirectories(parentPath string, namePrefix string, count int, onDir func(dirPath string)) {
	var err error
	for i := 0; i < count; i++ {
		dirName := fmt.Sprintf("%v_%v", namePrefix, i+1)
		dirPath := filepath.Join(parentPath, dirName)
		err = os.Mkdir(dirPath, 0777)
		if err != nil {
			panic(err)
		}
		if onDir != nil {
			onDir(dirPath)
		}
	}
}

func randomString(n int) []byte {
	bytes := make([]byte, n)
	for i := range bytes {
		bytes[i] = letters[rand.Intn(len(letters))]
	}
	return bytes
}

func (t *ReadonlyLoopbackFSTest) fillPhysicalFS() {
	var err error
	createDirectories(t.physicalPath, "top_dir", 10, func(dirPath string) {
		fileName := fmt.Sprintf("secondary_file.txt")
		contents := randomString(17)
		err = ioutil.WriteFile(filepath.Join(dirPath, fileName), contents, 0777)
		if err != nil {
			panic(err)
		}
		createDirectories(dirPath, "secondary_dir", 5, func(dirPath string) {
			for i := 0; i < 3; i++ {
				fileName := fmt.Sprintf("file_%v.txt", i+1)
				contents := randomString(i * 10)
				err = ioutil.WriteFile(filepath.Join(dirPath, fileName), contents, 0777)
				if err != nil {
					panic(err)
				}
			}
		})
	})
}

func (t *ReadonlyLoopbackFSTest) ListDirUsingWalk() {
	countedFiles, countedDirs := 0, 0
	err := filepath.Walk(t.Dir, func(path string, info os.FileInfo, err error) error {
		AssertNe(nil, info)
		if info.IsDir() {
			countedDirs++
		} else {
			if strings.Contains(path, "file_1.txt") {
				AssertEq(0, info.Size())
			} else {
				AssertTrue(info.Size() > 0)
			}
			countedFiles++
		}
		return nil
	})
	AssertEq(nil, err)
	AssertEq(1+10+10*5, countedDirs)
	AssertEq(10+10*5*3, countedFiles)
}

func (t *ReadonlyLoopbackFSTest) ListDirUsingDirectQuery() {
	infos, err := ioutil.ReadDir(filepath.Join(t.Dir, "top_dir_3"))
	AssertEq(nil, err)
	AssertEq(1+5, len(infos))
	for i := 0; i < 5; i++ {
		AssertEq(fmt.Sprintf("secondary_dir_%v", i+1), infos[i].Name())
		AssertTrue(infos[i].IsDir())
	}
	AssertEq("secondary_file.txt", infos[5].Name())
	AssertFalse(infos[5].IsDir())

	infos, err = ioutil.ReadDir(filepath.Join(t.Dir, "top_dir_4", "secondary_dir_1"))
	AssertEq(nil, err)
	AssertEq(3, len(infos))
	for i := 0; i < 3; i++ {
		AssertEq(fmt.Sprintf("file_%v.txt", i+1), infos[i].Name())
		AssertFalse(infos[i].IsDir())
	}
}

func (t *ReadonlyLoopbackFSTest) ReadFile() {
	bytes, err := ioutil.ReadFile(filepath.Join(t.Dir, "top_dir_1", "secondary_file.txt"))
	AssertEq(nil, err)
	AssertEq(17, len(bytes))

	bytes, err = ioutil.ReadFile(filepath.Join(t.Dir, "top_dir_1", "secondary_dir_3", "file_1.txt"))
	AssertEq(nil, err)
	AssertEq(0, len(bytes))

	bytes, err = ioutil.ReadFile(filepath.Join(t.Dir, "top_dir_1", "secondary_dir_3", "file_3.txt"))
	AssertEq(nil, err)
	AssertEq(20, len(bytes))
}
