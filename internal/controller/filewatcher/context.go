// Copyright 2025 Tom Barlow
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

package filewatcher

import (
	"path/filepath"
	"time"
)

// Context represents file event metadata made available to triggered workflows.
// It provides comprehensive information about the file that triggered the workflow,
// including path details, event type, and file metadata.
type Context struct {
	// Path is the absolute path to the file that triggered the event
	Path string `json:"path"`

	// Name is the filename without directory component
	Name string `json:"name"`

	// Dir is the directory containing the file
	Dir string `json:"dir"`

	// Ext is the file extension including the dot (e.g., ".pdf", ".txt")
	// Empty string if the file has no extension
	Ext string `json:"ext"`

	// Event is the type of filesystem event: created, modified, deleted, or renamed
	Event string `json:"event"`

	// OldPath contains the previous path for renamed events only
	// Empty string for all other event types
	OldPath string `json:"old_path,omitempty"`

	// Size is the file size in bytes
	// Zero for deleted events where file no longer exists
	Size int64 `json:"size,omitempty"`

	// MTime is the file modification time
	// Zero value for deleted events where file no longer exists
	MTime time.Time `json:"mtime,omitempty"`

	// IsDir indicates whether the event is for a directory
	IsDir bool `json:"is_dir"`
}

// NewContext creates a file watcher context from a file path and event type.
// It automatically populates path components and file metadata.
// For deleted events, size and mtime will be zero values.
func NewContext(path, event string, isDir bool, size int64, mtime time.Time) *Context {
	return &Context{
		Path:  path,
		Name:  filepath.Base(path),
		Dir:   filepath.Dir(path),
		Ext:   filepath.Ext(path),
		Event: event,
		Size:  size,
		MTime: mtime,
		IsDir: isDir,
	}
}
