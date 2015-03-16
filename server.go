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
	"io"
	"log"
	"time"

	"golang.org/x/net/context"

	bazilfuse "bazil.org/fuse"
)

// An object that terminates one end of the userspace <-> FUSE VFS connection.
type server struct {
	logger *log.Logger
	fs     FileSystem
}

// Create a server that relays requests to the supplied file system.
func newServer(fs FileSystem) (s *server, err error) {
	s = &server{
		logger: getLogger(),
		fs:     fs,
	}

	return
}

// Convert an absolute cache expiration time to a relative time from now for
// consumption by fuse.
func convertExpirationTime(t time.Time) (d time.Duration) {
	// Fuse represents durations as unsigned 64-bit counts of seconds and 32-bit
	// counts of nanoseconds (cf. http://goo.gl/EJupJV). The bazil.org/fuse
	// package converts time.Duration values to this form in a straightforward
	// way (cf. http://goo.gl/FJhV8j).
	//
	// So negative durations are right out. There is no need to cap the positive
	// magnitude, because 2^64 seconds is well longer than the 2^63 ns range of
	// time.Duration.
	d = t.Sub(time.Now())
	if d < 0 {
		d = 0
	}

	return
}

func convertChildInodeEntry(
	in *ChildInodeEntry,
	out *bazilfuse.LookupResponse) {
	out.Node = bazilfuse.NodeID(in.Child)
	out.Generation = uint64(in.Generation)
	out.Attr = convertAttributes(in.Child, in.Attributes)
	out.AttrValid = convertExpirationTime(in.AttributesExpiration)
	out.EntryValid = convertExpirationTime(in.EntryExpiration)
}

func convertHeader(
	in bazilfuse.Header) (out RequestHeader) {
	out.Uid = in.Uid
	out.Gid = in.Gid
	return
}

// Serve the fuse connection by repeatedly reading requests from the supplied
// FUSE connection, responding as dictated by the file system. Return when the
// connection is closed or an unexpected error occurs.
func (s *server) Serve(c *bazilfuse.Conn) (err error) {
	// Read a message at a time, dispatching to goroutines doing the actual
	// processing.
	for {
		var fuseReq bazilfuse.Request
		fuseReq, err = c.ReadRequest()

		// ReadRequest returns EOF when the connection has been closed.
		if err == io.EOF {
			err = nil
			return
		}

		// Otherwise, forward on errors.
		if err != nil {
			err = fmt.Errorf("Conn.ReadRequest: %v", err)
			return
		}

		go s.handleFuseRequest(fuseReq)
	}
}

func (s *server) handleFuseRequest(fuseReq bazilfuse.Request) {
	// Log the request.
	s.logger.Println("Received:", fuseReq)

	// TODO(jacobsa): Support cancellation when interrupted, if we can coax the
	// system into reproducing such requests.
	ctx := context.Background()

	// Attempt to handle it.
	switch typed := fuseReq.(type) {
	case *bazilfuse.InitRequest:
		// Convert the request.
		req := &InitRequest{
			Header: convertHeader(typed.Header),
		}

		// Call the file system.
		_, err := s.fs.Init(ctx, req)
		if err != nil {
			s.logger.Println("Responding:", err)
			typed.RespondError(err)
			return
		}

		// Convert the response.
		fuseResp := &bazilfuse.InitResponse{}
		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	case *bazilfuse.StatfsRequest:
		// Responding to this is required to make mounting work, at least on OS X.
		// We don't currently expose the capability for the file system to
		// intercept this.
		fuseResp := &bazilfuse.StatfsResponse{}
		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	case *bazilfuse.LookupRequest:
		// Convert the request.
		req := &LookUpInodeRequest{
			Header: convertHeader(typed.Header),
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
		}

		// Call the file system.
		resp, err := s.fs.LookUpInode(ctx, req)
		if err != nil {
			s.logger.Println("Responding:", err)
			typed.RespondError(err)
			return
		}

		// Convert the response.
		fuseResp := &bazilfuse.LookupResponse{}
		convertChildInodeEntry(&resp.Entry, fuseResp)

		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	case *bazilfuse.GetattrRequest:
		// Convert the request.
		req := &GetInodeAttributesRequest{
			Header: convertHeader(typed.Header),
			Inode:  InodeID(typed.Header.Node),
		}

		// Call the file system.
		resp, err := s.fs.GetInodeAttributes(ctx, req)
		if err != nil {
			s.logger.Println("Responding:", err)
			typed.RespondError(err)
			return
		}

		// Convert the response.
		fuseResp := &bazilfuse.GetattrResponse{
			Attr:      convertAttributes(req.Inode, resp.Attributes),
			AttrValid: convertExpirationTime(resp.AttributesExpiration),
		}

		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	case *bazilfuse.SetattrRequest:
		// Convert the request.
		req := &SetInodeAttributesRequest{
			Header: convertHeader(typed.Header),
			Inode:  InodeID(typed.Header.Node),
		}

		if typed.Valid&bazilfuse.SetattrSize != 0 {
			req.Size = &typed.Size
		}

		if typed.Valid&bazilfuse.SetattrMode != 0 {
			req.Mode = &typed.Mode
		}

		if typed.Valid&bazilfuse.SetattrAtime != 0 {
			req.Atime = &typed.Atime
		}

		if typed.Valid&bazilfuse.SetattrMtime != 0 {
			req.Mtime = &typed.Mtime
		}

		// Call the file system.
		resp, err := s.fs.SetInodeAttributes(ctx, req)
		if err != nil {
			s.logger.Println("Responding:", err)
			typed.RespondError(err)
			return
		}

		// Convert the response.
		fuseResp := &bazilfuse.SetattrResponse{
			Attr:      convertAttributes(req.Inode, resp.Attributes),
			AttrValid: convertExpirationTime(resp.AttributesExpiration),
		}

		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	case *bazilfuse.MkdirRequest:
		// Convert the request.
		req := &MkDirRequest{
			Header: convertHeader(typed.Header),
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
			Mode:   typed.Mode,
		}

		// Call the file system.
		resp, err := s.fs.MkDir(ctx, req)
		if err != nil {
			s.logger.Println("Responding:", err)
			typed.RespondError(err)
			return
		}

		// Convert the response.
		fuseResp := &bazilfuse.MkdirResponse{}
		convertChildInodeEntry(&resp.Entry, &fuseResp.LookupResponse)

		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	case *bazilfuse.CreateRequest:
		// Convert the request.
		req := &CreateFileRequest{
			Header: convertHeader(typed.Header),
			Parent: InodeID(typed.Header.Node),
			Name:   typed.Name,
			Mode:   typed.Mode,
			Flags:  typed.Flags,
		}

		// Call the file system.
		resp, err := s.fs.CreateFile(ctx, req)
		if err != nil {
			s.logger.Println("Responding:", err)
			typed.RespondError(err)
			return
		}

		// Convert the response.
		fuseResp := &bazilfuse.CreateResponse{
			OpenResponse: bazilfuse.OpenResponse{
				Handle: bazilfuse.HandleID(resp.Handle),
			},
		}
		convertChildInodeEntry(&resp.Entry, &fuseResp.LookupResponse)

		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	case *bazilfuse.RemoveRequest:
		if typed.Dir {
			// Convert the request.
			req := &RmDirRequest{
				Header: convertHeader(typed.Header),
				Parent: InodeID(typed.Header.Node),
				Name:   typed.Name,
			}

			// Call the file system.
			_, err := s.fs.RmDir(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Respond successfully.
			s.logger.Println("Responding OK.")
			typed.Respond()
		} else {
			// Convert the request.
			req := &UnlinkRequest{
				Header: convertHeader(typed.Header),
				Parent: InodeID(typed.Header.Node),
				Name:   typed.Name,
			}

			// Call the file system.
			_, err := s.fs.Unlink(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Respond successfully.
			s.logger.Println("Responding OK.")
			typed.Respond()
		}

	case *bazilfuse.OpenRequest:
		// Directory or file?
		if typed.Dir {
			// Convert the request.
			req := &OpenDirRequest{
				Header: convertHeader(typed.Header),
				Inode:  InodeID(typed.Header.Node),
				Flags:  typed.Flags,
			}

			// Call the file system.
			resp, err := s.fs.OpenDir(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Convert the response.
			fuseResp := &bazilfuse.OpenResponse{
				Handle: bazilfuse.HandleID(resp.Handle),
			}

			s.logger.Println("Responding:", fuseResp)
			typed.Respond(fuseResp)
		} else {
			// Convert the request.
			req := &OpenFileRequest{
				Header: convertHeader(typed.Header),
				Inode:  InodeID(typed.Header.Node),
				Flags:  typed.Flags,
			}

			// Call the file system.
			resp, err := s.fs.OpenFile(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Convert the response.
			fuseResp := &bazilfuse.OpenResponse{
				Handle: bazilfuse.HandleID(resp.Handle),
			}

			s.logger.Println("Responding:", fuseResp)
			typed.Respond(fuseResp)
		}

	case *bazilfuse.ReadRequest:
		// Directory or file?
		if typed.Dir {
			// Convert the request.
			req := &ReadDirRequest{
				Header: convertHeader(typed.Header),
				Inode:  InodeID(typed.Header.Node),
				Handle: HandleID(typed.Handle),
				Offset: DirOffset(typed.Offset),
				Size:   typed.Size,
			}

			// Call the file system.
			resp, err := s.fs.ReadDir(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Convert the response.
			fuseResp := &bazilfuse.ReadResponse{
				Data: resp.Data,
			}

			s.logger.Println("Responding:", fuseResp)
			typed.Respond(fuseResp)
		} else {
			// Convert the request.
			req := &ReadFileRequest{
				Header: convertHeader(typed.Header),
				Inode:  InodeID(typed.Header.Node),
				Handle: HandleID(typed.Handle),
				Offset: typed.Offset,
				Size:   typed.Size,
			}

			// Call the file system.
			resp, err := s.fs.ReadFile(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Convert the response.
			fuseResp := &bazilfuse.ReadResponse{
				Data: resp.Data,
			}

			s.logger.Println("Responding:", fuseResp)
			typed.Respond(fuseResp)
		}

	case *bazilfuse.ReleaseRequest:
		// Directory or file?
		if typed.Dir {
			// Convert the request.
			req := &ReleaseDirHandleRequest{
				Header: convertHeader(typed.Header),
				Handle: HandleID(typed.Handle),
			}

			// Call the file system.
			_, err := s.fs.ReleaseDirHandle(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Respond successfully.
			s.logger.Println("Responding OK.")
			typed.Respond()
		} else {
			// Convert the request.
			req := &ReleaseFileHandleRequest{
				Header: convertHeader(typed.Header),
				Handle: HandleID(typed.Handle),
			}

			// Call the file system.
			_, err := s.fs.ReleaseFileHandle(ctx, req)
			if err != nil {
				s.logger.Println("Responding:", err)
				typed.RespondError(err)
				return
			}

			// Respond successfully.
			s.logger.Println("Responding OK.")
			typed.Respond()
		}

	case *bazilfuse.WriteRequest:
		// Convert the request.
		req := &WriteFileRequest{
			Header: convertHeader(typed.Header),
			Inode:  InodeID(typed.Header.Node),
			Handle: HandleID(typed.Handle),
			Data:   typed.Data,
			Offset: typed.Offset,
		}

		// Call the file system.
		_, err := s.fs.WriteFile(ctx, req)
		if err != nil {
			s.logger.Println("Responding:", err)
			typed.RespondError(err)
			return
		}

		// Convert the response.
		fuseResp := &bazilfuse.WriteResponse{
			Size: len(typed.Data),
		}

		s.logger.Println("Responding:", fuseResp)
		typed.Respond(fuseResp)

	default:
		s.logger.Println("Unhandled type. Returning ENOSYS.")
		typed.RespondError(ENOSYS)
	}
}

func convertAttributes(inode InodeID, attr InodeAttributes) bazilfuse.Attr {
	return bazilfuse.Attr{
		Inode:  uint64(inode),
		Size:   attr.Size,
		Mode:   attr.Mode,
		Atime:  attr.Atime,
		Mtime:  attr.Mtime,
		Ctime:  attr.Ctime,
		Crtime: attr.Crtime,
		Uid:    attr.Uid,
		Gid:    attr.Gid,
	}
}
