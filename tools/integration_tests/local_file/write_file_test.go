// Copyright 2023 Google LLC
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

// Provides integration tests for write on local files.
package local_file_test

import (
	"testing"

	. "github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/client"
	"github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/operations"
	"github.com/googlecloudplatform/gcsfuse/v2/tools/integration_tests/util/setup"
)

func TestMultipleWritesToLocalFile(t *testing.T) {
	testDirPath = setup.SetupTestDirectory(testDirName)
	// Create a local file.
	_, fh := CreateLocalFileInTestDir(ctx, storageClient, testDirPath, FileName1, t)

	// Write some contents to file sequentially.
	for i := 0; i < 3; i++ {
		operations.WriteWithoutClose(fh, FileContents, t)
	}
	ValidateObjectNotFoundErrOnGCS(ctx, storageClient, testDirName, FileName1, t)

	// Close the file and validate that the file is created on GCS.
	CloseFileAndValidateContentFromGCS(ctx, storageClient, fh, testDirName,
		FileName1, FileContents+FileContents+FileContents, t)
}

func TestRandomWritesToLocalFile(t *testing.T) {
	testDirPath = setup.SetupTestDirectory(testDirName)
	// Create a local file.
	_, fh := CreateLocalFileInTestDir(ctx, storageClient, testDirPath, FileName1, t)

	// Write some contents to file randomly.
	operations.WriteAt("string1", 0, fh, t)
	operations.WriteAt("string2", 2, fh, t)
	operations.WriteAt("string3", 3, fh, t)
	ValidateObjectNotFoundErrOnGCS(ctx, storageClient, testDirName, FileName1, t)

	// Close the file and validate that the file is created on GCS.
	CloseFileAndValidateContentFromGCS(ctx, storageClient, fh, testDirName,
		FileName1, "stsstring3", t)
}

func TestOutOfOrderWritesToNewFile(t *testing.T) {
	testDirPath = setup.SetupTestDirectory(testDirName)
	// Create a local file.
	_, fh := CreateLocalFileInTestDir(ctx, storageClient, testDirPath, FileName1, t)

	// Write some contents to file sequentially.
	for i := 0; i < 2; i++ {
		operations.WriteWithoutClose(fh, FileContents, t)
	}
	ValidateObjectNotFoundErrOnGCS(ctx, storageClient, testDirName, FileName1, t)

	// Write at previous offset.
	operations.WriteAt("hello", 0, fh, t)

	expectedString := "hellotringtestString"
	// Close the file and validate that the file is created on GCS.
	CloseFileAndValidateContentFromGCS(ctx, storageClient, fh, testDirName,
		FileName1, expectedString, t)
}

func TestMultipleOutOfOrderWritesToNewFile(t *testing.T) {
	testDirPath = setup.SetupTestDirectory(testDirName)
	// Create a local file.
	_, fh := CreateLocalFileInTestDir(ctx, storageClient, testDirPath, FileName1, t)

	// Write some contents to file sequentially.
	for i := 0; i < 2; i++ {
		operations.WriteWithoutClose(fh, FileContents, t)
	}
	ValidateObjectNotFoundErrOnGCS(ctx, storageClient, testDirName, FileName1, t)

	// Write at previous offset.
	operations.WriteAt("hello", 15, fh, t)

	// Write at new offset.
	operations.WriteAt("hey", 30, fh, t)

	emptyBytes := [10]byte{}
	expectedString := "testStringtestShello" + string(emptyBytes[:]) + "hey"
	// Close the file and validate that the file is created on GCS.
	CloseFileAndValidateContentFromGCS(ctx, storageClient, fh, testDirName,
		FileName1, expectedString, t)
}

func TestWritesToNewFileStartingAtNonZeroOffset(t *testing.T) {
	testDirPath = setup.SetupTestDirectory(testDirName)
	// Create a local file.
	_, fh := CreateLocalFileInTestDir(ctx, storageClient, testDirPath, FileName1, t)
	// Write at future offset.
	operations.WriteAt("hello", 15, fh, t)
	// Write at zero offset now.
	operations.WriteAt("hey", 0, fh, t)

	emptyBytes := [12]byte{}
	expectedString := "hey" + string(emptyBytes[:]) + "hello"
	// Close the file and validate that the file is created on GCS.
	CloseFileAndValidateContentFromGCS(ctx, storageClient, fh, testDirName,
		FileName1, expectedString, t)
}
