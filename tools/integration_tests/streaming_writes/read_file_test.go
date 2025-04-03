// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package streaming_writes

import (
	"log"
	"path"
	"time"

	. "github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/client"
	"github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (t *defaultMountCommonTest) TestReadFileSucceedsForZB() {
	// Write some content to the file.
	_, err := t.f1.WriteAt([]byte(t.data), 0)
	assert.NoError(t.T(), err)
	// Sync File to ensure buffers are flushed to GCS.
	operations.SyncFile(t.f1, t.T())

	statRes, err := operations.StatFile(t.filePath)

	require.NoError(t.T(), err)
	assert.Equal(t.T(), t.fileName, (*statRes).Name())
	assert.EqualValues(t.T(), len(t.data), (*statRes).Size())

	// Reading the file contents.
	buf := make([]byte, len(t.data))
	for i := 0; i < 10; i++ {
		time.Sleep(5 * time.Second)
		_, err = t.f1.ReadAt(buf, 0)
		log.Printf("Read error: %v", err)
	}
	_, err = t.f1.ReadAt(buf, 0)

	t.validateReadSucceedsForZB(err)
	// Close the file and validate that the file is created on GCS.
	CloseFileAndValidateContentFromGCS(ctx, storageClient, t.f1, testDirName, t.fileName, t.data, t.T())
}

func (t *defaultMountCommonTest) TestReadBeforeFileIsFlushed() {
	testContent := "testContent"
	// Write data to file.
	operations.WriteAt(testContent, 0, t.f1, t.T())

	// Try to read the file.
	_, err := t.f1.Seek(0, 0)
	require.NoError(t.T(), err)
	buf := make([]byte, 10)
	_, err = t.f1.Read(buf)

	require.Error(t.T(), err, "input/output error")
	// Validate if correct content is uploaded to GCS after read error.
	CloseFileAndValidateContentFromGCS(ctx, storageClient, t.f1, testDirName, t.fileName, testContent, t.T())
}

func (t *defaultMountCommonTest) TestReadAfterFlush() {
	testContent := "testContent"
	// Write data to file and flush.
	operations.WriteAt(testContent, 0, t.f1, t.T())
	CloseFileAndValidateContentFromGCS(ctx, storageClient, t.f1, testDirName, t.fileName, testContent, t.T())

	// Perform read and validate the contents.
	var err error
	t.f1, err = operations.OpenFileAsReadonly(path.Join(testDirPath, t.fileName))
	require.NoError(t.T(), err)
	buf := make([]byte, len(testContent))
	_, err = t.f1.Read(buf)

	require.NoError(t.T(), err)
	require.Equal(t.T(), string(buf), testContent)
}
