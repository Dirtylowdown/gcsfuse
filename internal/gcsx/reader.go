// Copyright 2025 Google LLC
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

package gcsx

import (
	"context"

	"github.com/googlecloudplatform/gcsfuse/v2/internal/gcsx/readers"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/storage/gcs"
)

// Ensures internal consistency of the implementation
type InvariantChecker interface {
	CheckInvariants()
}

// Provides methods to read data at a specific offset
type DataReaderWithEnd interface {
	ReadAt(ctx context.Context, p []byte, offset, end int64) (objectData readers.ObjectData, err error)
}

// Provides methods to read data at a specific offset
type DataReader interface {
	ReadAt(ctx context.Context, p []byte, offset int64) (objectData readers.ObjectData, err error)
}

// Provides access to object metadata
type ObjectAccessor interface {
	Object() (o *gcs.MinObject)
}

// Handles cleanup of resources
type Destroyer interface {
	Destroy()
}

// Combines all interfaces for a complete reader
type Reader interface {
	InvariantChecker
	DataReader
	ObjectAccessor
	Destroyer
}
