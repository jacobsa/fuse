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
	"github.com/jacobsa/fuse/internal/buffer"
)

////////////////////////////////////////////////////////////////////////
// buffer.InMessage
////////////////////////////////////////////////////////////////////////

func (c *Connection) getInMessage() *buffer.InMessage {
	return c.inMessages.Get().(*buffer.InMessage)
}

func (c *Connection) putInMessage(x *buffer.InMessage) {
	c.inMessages.Put(x)
}

////////////////////////////////////////////////////////////////////////
// buffer.OutMessage
////////////////////////////////////////////////////////////////////////

func (c *Connection) getOutMessage() *buffer.OutMessage {
	return c.outMessages.Get().(*buffer.OutMessage)
}

func (c *Connection) putOutMessage(x *buffer.OutMessage) {
	x.Reset()
	c.outMessages.Put(x)
}
