// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stale_handle

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	. "github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/client"
	"github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/operations"
	"github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// //////////////////////////////////////////////////////////////////////
// Boilerplate
// //////////////////////////////////////////////////////////////////////

type staleFileHandleCommon struct {
	f1          *os.File
	data        string
	testDirName string
	testDirPath string
	// Directory to unmount.
	mntDir string
	flags  []string
	suite.Suite
}

// //////////////////////////////////////////////////////////////////////
// Helpers
// //////////////////////////////////////////////////////////////////////

func getTestName(t *testing.T) string {
	return strings.ReplaceAll(t.Name(), "/", "_")
}

func (s *staleFileHandleCommon) SetupSuite() {
	operations.CreateDirectory(path.Join(setup.TestDir(), getTestName(s.T())), s.T())
	s.mntDir = path.Join(setup.TestDir(), getTestName(s.T()), "mnt")
	operations.CreateDirectory(s.mntDir, s.T())
	setup.MountGCSFuseWithGivenMountFuncMntDirAndLogFile(s.flags, s.mntDir, path.Join(setup.TestDir(), getTestName(s.T()), "gcsfuse.log"), mountFunc)
	s.testDirName = getTestName(s.T())
	s.testDirPath = setup.SetupTestDirectoryOnMntDir(s.mntDir, s.testDirName)
}

func (s *staleFileHandleCommon) TearDownSuite() {
	operations.RemoveDir(s.testDirPath)
	setup.SaveGCSFuseLogFileInCaseOfFailureGivenLogFile(s.T(), path.Join(setup.TestDir(), getTestName(s.T()), "gcsfuse.log"))
}

func (s *staleFileHandleCommon) TearDownTest() {
	setup.CleanupDirectoryOnGCS(ctx, storageClient, s.testDirName)
}

func (s *staleFileHandleCommon) streamingWritesEnabled() bool {
	s.T().Helper()
	return slices.Contains(s.flags, "--enable-streaming-writes=true")
}

// Used to validate stale handle error from sync/close when streaming writes are disabled.
func (s *staleFileHandleCommon) validateStaleNFSFileHandleErrorIfStreamingWritesDisabled(err error) {
	s.T().Helper()
	if !s.streamingWritesEnabled() {
		operations.ValidateStaleNFSFileHandleError(s.T(), err)
	} else {
		assert.NoError(s.T(), err)
	}
}

////////////////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////////////////

func (s *staleFileHandleCommon) TestClobberedFileSyncAndCloseThrowsStaleFileHandleError() {
	// Dirty the file by giving it some contents.
	_, err := s.f1.WriteAt([]byte(s.data), 0)
	assert.NoError(s.T(), err)
	// Clobber file by replacing the underlying object with a new generation.
	err = WriteToObject(ctx, storageClient, path.Join(s.testDirName, FileName1), FileContents, storage.Conditions{})
	assert.NoError(s.T(), err)

	err = s.f1.Sync()

	s.validateStaleNFSFileHandleErrorIfStreamingWritesDisabled(err)
	// Closing the file/writer returns stale NFS file handle error.
	err = s.f1.Close()
	operations.ValidateStaleNFSFileHandleError(s.T(), err)
	ValidateObjectContentsFromGCS(ctx, storageClient, s.testDirName, FileName1, FileContents, s.T())
}

func (s *staleFileHandleCommon) TestFileDeletedLocallySyncAndCloseDoNotThrowError() {
	// Dirty the file by giving it some contents.
	bytesWrote, err := s.f1.WriteAt([]byte(s.data), 0)
	assert.NoError(s.T(), err)
	// Delete the file.
	operations.RemoveFile(s.f1.Name())
	// Verify unlink operation succeeds.

	operations.ValidateNoFileOrDirError(s.T(), s.f1.Name())
	// Attempt to write to file should not give any error.
	_, err = s.f1.WriteAt([]byte(s.data), int64(bytesWrote))

	assert.NoError(s.T(), err)
	operations.SyncFile(s.f1, s.T())
	operations.CloseFileShouldNotThrowError(s.f1, s.T())
}
