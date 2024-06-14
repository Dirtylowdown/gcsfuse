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

package fs_test

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/googlecloudplatform/gcsfuse/v2/internal/cache/lru"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/cache/metadata"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/fs/inode"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/mount"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/storage/caching"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/storage/fake"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/storage/gcs"
	"github.com/googlecloudplatform/gcsfuse/v2/internal/storage/storageutil"
	"github.com/jacobsa/fuse/fusetesting"
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
	"github.com/jacobsa/timeutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestCachingTestSuite(t *testing.T) {
	suite.Run(t, new(CachingTest))
	suite.Run(t, new(CachingWithImplicitDirsTest))
	suite.Run(t, new(MultiBucketMountCachingTest))
}

////////////////////////////////////////////////////////////////////////
// Common
////////////////////////////////////////////////////////////////////////

const ttl = 10 * time.Minute

var (
	uncachedBucket gcs.Bucket
)

func newLruCache(capacity uint64) *lru.Cache {
	return lru.NewCache(capacity)
}

type cachingTestCommon struct {
	fsTest
}

func (t *cachingTestCommon) SetupSuite() {
	// Wrap the bucket in a stat caching layer for the purposes of the file
	// system.
	uncachedBucket = fake.NewFakeBucket(timeutil.RealClock(), "some_bucket")
	lruCache := newLruCache(uint64(1000 * mount.AverageSizeOfPositiveStatCacheEntry))
	statCache := metadata.NewStatCacheBucketView(lruCache, "")
	bucket = caching.NewFastStatBucket(
		ttl,
		statCache,
		&cacheClock,
		uncachedBucket)

	// Enable directory type caching.
	t.serverCfg.DirTypeCacheTTL = ttl

	// Call through.
	t.fsTest.SetupSuite()
}

////////////////////////////////////////////////////////////////////////
// Caching
////////////////////////////////////////////////////////////////////////

type CachingTest struct {
	suite.Suite
	suite.SetupAllSuite
	suite.TearDownAllSuite
	suite.TearDownTestSuite
	cachingTestCommon
}

func (t *CachingTest) SetupSuite() {
	t.cachingTestCommon.SetupSuite()
}

func (t *CachingTest) TearDownSuite() {
	t.cachingTestCommon.TearDownSuite()
}

func (t *CachingTest) TearDownTest() {
	t.cachingTestCommon.TearDownTest()
}

func (t *CachingTest) TestEmptyBucket() {
	// ReadDir
	entries, err := fusetesting.ReadDirPicky(mntDir)
	assert.Nil(t.T(), err)

	ExpectThat(entries, ElementsAre())
}

func (t *CachingTest) TestFileCreatedRemotely() {
	const name = "foo"
	const contents = "taco"

	var fi os.FileInfo

	// Create an object in GCS.
	_, err := storageutil.CreateObject(
		ctx,
		uncachedBucket,
		name,
		[]byte(contents))

	assert.Nil(t.T(), err)

	// It should immediately show up in a listing.
	entries, err := fusetesting.ReadDirPicky(mntDir)
	assert.Nil(t.T(), err)
	AssertEq(1, len(entries))

	fi = entries[0]
	ExpectEq(name, fi.Name())
	ExpectEq(len(contents), fi.Size())

	// And we should be able to stat it.
	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)

	ExpectEq(name, fi.Name())
	ExpectEq(len(contents), fi.Size())

	// And read it.
	b, err := ioutil.ReadFile(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq(contents, string(b))

	// And overwrite it, and read it back again.
	err = ioutil.WriteFile(path.Join(mntDir, name), []byte("burrito"), 0500)
	assert.Nil(t.T(), err)

	b, err = ioutil.ReadFile(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq("burrito", string(b))
}

func (t *CachingTest) TestFileChangedRemotely() {
	const name = "foo"
	var fi os.FileInfo
	var err error

	// Create a file via the file system.
	err = ioutil.WriteFile(path.Join(mntDir, name), []byte("taco"), 0500)
	assert.Nil(t.T(), err)

	// Overwrite the object in GCS.
	_, err = storageutil.CreateObject(
		ctx,
		uncachedBucket,
		name,
		[]byte("burrito"))

	assert.Nil(t.T(), err)

	// Because we are caching, the file should still appear to be the local
	// version.
	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq(len("taco"), fi.Size())

	// After the TTL elapses, we should see the new version.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq(len("burrito"), fi.Size())

	// Reading should work as expected.
	b, err := ioutil.ReadFile(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq("burrito", string(b))
}

func (t *CachingTest) TestDirectoryRemovedRemotely() {
	const name = "foo"
	var fi os.FileInfo
	var err error

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(mntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Remove the backing object in GCS.
	err = uncachedBucket.DeleteObject(
		ctx,
		&gcs.DeleteObjectRequest{Name: name + "/"})

	assert.Nil(t.T(), err)

	// Because we are caching, the directory should still appear to exist.
	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectTrue(fi.IsDir())

	// After the TTL elapses, we should see it disappear.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	_, err = os.Stat(path.Join(mntDir, name))
	ExpectTrue(os.IsNotExist(err), "err: %v", err)
}

func (t *CachingTest) TestConflictingNames_RemoteModifier() {
	const name = "foo"
	var fi os.FileInfo
	var err error

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(mntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Create a file with the same name via GCS.
	_, err = storageutil.CreateObject(
		ctx,
		uncachedBucket,
		name,
		[]byte("taco"))

	assert.Nil(t.T(), err)

	// Because the file system is caching types, it will fail to find the file
	// when statting.
	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectTrue(fi.IsDir())

	_, err = os.Stat(path.Join(mntDir, name+inode.ConflictingFileNameSuffix))
	ExpectTrue(os.IsNotExist(err), "err: %v", err)

	// After the TTL elapses, we should see both.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectTrue(fi.IsDir())

	fi, err = os.Stat(path.Join(mntDir, name+inode.ConflictingFileNameSuffix))
	assert.Nil(t.T(), err)
	ExpectFalse(fi.IsDir())
}

func (t *CachingTest) TestTypeOfNameChanges_LocalModifier() {
	const name = "test"
	var fi os.FileInfo
	var err error

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(mntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Delete it and recreate as a file.
	err = os.Remove(path.Join(mntDir, name))
	assert.Nil(t.T(), err)

	err = ioutil.WriteFile(path.Join(mntDir, name), []byte("taco"), 0400)
	assert.Nil(t.T(), err)

	// All caches should have been updated.
	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectFalse(fi.IsDir())
	ExpectEq(len("taco"), fi.Size())
}

func (t *CachingTest) TestTypeOfNameChanges_RemoteModifier() {
	const name = "foo"
	var fi os.FileInfo
	var err error

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(mntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Remove the backing object in GCS, updating the bucket cache (but not the
	// file system type cache)
	err = bucket.DeleteObject(
		ctx,
		&gcs.DeleteObjectRequest{Name: name + "/"})

	assert.Nil(t.T(), err)

	// Create a file with the same name via GCS, again updating the bucket cache.
	_, err = storageutil.CreateObject(
		ctx,
		bucket,
		name,
		[]byte("taco"))

	assert.Nil(t.T(), err)

	// Because the file system is caching types, it will fail to find the name.
	_, err = os.Stat(path.Join(mntDir, name))
	ExpectTrue(os.IsNotExist(err), "err: %v", err)

	// After the TTL elapses, we should see it turn into a file.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	fi, err = os.Stat(path.Join(mntDir, name))
	assert.Nil(t.T(), err)
	ExpectFalse(fi.IsDir())
}

////////////////////////////////////////////////////////////////////////
// Caching with implicit directories
////////////////////////////////////////////////////////////////////////

type CachingWithImplicitDirsTest struct {
	suite.Suite
	suite.SetupAllSuite
	suite.TearDownAllSuite
	suite.TearDownTestSuite
	cachingTestCommon
}

func (t *CachingWithImplicitDirsTest) SetupSuite() {
	t.serverCfg.ImplicitDirectories = true
	t.cachingTestCommon.SetupSuite()
}

func (t *CachingWithImplicitDirsTest) TearDownSuite() {
	t.cachingTestCommon.TearDownSuite()
}

func (t *CachingWithImplicitDirsTest) TearDownTest() {
	t.cachingTestCommon.TearDownTest()
}

func (t *CachingWithImplicitDirsTest) TestImplicitDirectory_DefinedByFile() {
	var fi os.FileInfo
	var err error

	// Set up a file object implicitly defining a directory in GCS.
	_, err = storageutil.CreateObject(
		ctx,
		uncachedBucket,
		"foo/bar",
		[]byte(""))

	assert.Nil(t.T(), err)

	// The directory should appear to exist.
	fi, err = os.Stat(path.Join(mntDir, "foo"))
	assert.Nil(t.T(), err)

	ExpectEq("foo", fi.Name())
	ExpectTrue(fi.IsDir())
}

func (t *CachingWithImplicitDirsTest) TestImplicitDirectory_DefinedByDirectory() {
	var fi os.FileInfo
	var err error

	// Set up a directory object implicitly defining a directory in GCS.
	_, err = storageutil.CreateObject(
		ctx,
		uncachedBucket,
		"foo/bar/",
		[]byte(""))

	assert.Nil(t.T(), err)

	// The directory should appear to exist.
	fi, err = os.Stat(path.Join(mntDir, "foo"))
	assert.Nil(t.T(), err)

	ExpectEq("foo", fi.Name())
	ExpectTrue(fi.IsDir())
}

func (t *CachingWithImplicitDirsTest) TestSymlinksWork() {
	var fi os.FileInfo
	var err error

	// Create a file.
	fileName := path.Join(mntDir, "foo")
	const contents = "taco"

	err = ioutil.WriteFile(fileName, []byte(contents), 0400)
	assert.Nil(t.T(), err)

	// Create a symlink to it.
	symlinkName := path.Join(mntDir, "bar")
	err = os.Symlink("foo", symlinkName)
	assert.Nil(t.T(), err)

	// Stat the link.
	fi, err = os.Lstat(symlinkName)
	assert.Nil(t.T(), err)

	ExpectEq("bar", fi.Name())
	ExpectEq(0, fi.Size())
	ExpectEq(filePerms|os.ModeSymlink, fi.Mode())

	// Stat the target via the link.
	fi, err = os.Stat(symlinkName)
	assert.Nil(t.T(), err)

	ExpectEq("bar", fi.Name())
	ExpectEq(len(contents), fi.Size())
	ExpectEq(filePerms, fi.Mode())
}

func (t *CachingWithImplicitDirsTest) TestSymlinksAreTypeCached() {
	var fi os.FileInfo
	var err error

	// Create a symlink.
	symlinkName := path.Join(mntDir, "foo")
	err = os.Symlink("blah", symlinkName)
	assert.Nil(t.T(), err)

	// Create a directory object out of band, so the root inode doesn't notice.
	_, err = storageutil.CreateObject(
		ctx,
		uncachedBucket,
		"foo/",
		[]byte(""))

	assert.Nil(t.T(), err)

	// The directory should not yet be visible, because the root inode should
	// have cached that the symlink is present under the name "foo".
	fi, err = os.Lstat(path.Join(mntDir, "foo"))

	assert.Nil(t.T(), err)
	ExpectEq(filePerms|os.ModeSymlink, fi.Mode())

	// After the TTL elapses, we should see the directory.
	cacheClock.AdvanceTime(ttl + time.Millisecond)
	fi, err = os.Lstat(path.Join(mntDir, "foo"))

	assert.Nil(t.T(), err)
	ExpectEq(dirPerms|os.ModeDir, fi.Mode())

	// And should be able to stat the symlink under the alternative name.
	fi, err = os.Lstat(path.Join(mntDir, "foo"+inode.ConflictingFileNameSuffix))

	assert.Nil(t.T(), err)
	ExpectEq("foo"+inode.ConflictingFileNameSuffix, fi.Name())
	ExpectEq(filePerms|os.ModeSymlink, fi.Mode())
}

////////////////////////////////////////////////////////////////////////
// Multi-bucket mount tests
////////////////////////////////////////////////////////////////////////

const (
	bucket1Name string = "fruits"
	bucket2Name string = "spices"
)

var (
	uncachedBuckets map[string]gcs.Bucket
)

type MultiBucketMountCachingTest struct {
	suite.Suite
	suite.SetupAllSuite
	suite.TearDownAllSuite
	suite.TearDownTestSuite
	fsTest
}

func getMultiMountBucketDir(bucketName string) string {
	return mntDir + "/" + bucketName
}

func (t *MultiBucketMountCachingTest) SetupSuite() {
	sharedCache := newLruCache(uint64(1000 * mount.AverageSizeOfPositiveStatCacheEntry))
	uncachedBuckets = make(map[string]gcs.Bucket)
	buckets = make(map[string]gcs.Bucket)

	// Create uncached buckets and wrap them in stat caching layer
	// for the purposes of the file system.
	for _, bucketName := range []string{bucket1Name, bucket2Name} {
		uncachedBuckets[bucketName] = fake.NewFakeBucket(timeutil.RealClock(), bucketName)
		statCache := metadata.NewStatCacheBucketView(sharedCache, bucketName)
		buckets[bucketName] = caching.NewFastStatBucket(
			ttl,
			statCache,
			&cacheClock,
			uncachedBuckets[bucketName])
	}

	// Enable directory type caching.
	t.serverCfg.DirTypeCacheTTL = ttl

	// Call through.
	t.fsTest.SetupSuite()
}

func (t *MultiBucketMountCachingTest) TearDownSuite() {
	t.fsTest.TearDownSuite()
}

func (t *MultiBucketMountCachingTest) TearDownTest() {
	for _, bucketName := range []string{bucket1Name, bucket2Name} {
		bucket := buckets[bucketName]
		AssertEq(nil, storageutil.DeleteAllObjects(context.Background(), bucket))
	}

	t.fsTest.TearDownTest()
}

func (t *MultiBucketMountCachingTest) TestBucketsAreEmptyInitially() {
	// ReadDir
	for _, bucketName := range []string{bucket1Name, bucket2Name} {
		entries, err := fusetesting.ReadDirPicky(getMultiMountBucketDir(bucketName))
		assert.Nil(t.T(), err)

		ExpectThat(entries, ElementsAre())
	}
}

func (t *MultiBucketMountCachingTest) TestFileCreatedRemotely() {
	const name = "foo"
	const contents = "taco"
	bucket1MntDir := getMultiMountBucketDir(bucket1Name)
	bucket2MntDir := getMultiMountBucketDir(bucket2Name)
	bucket1 := uncachedBuckets[bucket1Name]

	var fi os.FileInfo

	// Create an object in GCS.
	_, err := storageutil.CreateObject(
		ctx,
		bucket1,
		name,
		[]byte(contents))

	assert.Nil(t.T(), err)

	// It should immediately show up in a listing.
	entries, err := fusetesting.ReadDirPicky(bucket1MntDir)
	assert.Nil(t.T(), err)
	AssertEq(1, len(entries))

	fi = entries[0]
	ExpectEq(name, fi.Name())
	ExpectEq(len(contents), fi.Size())

	// we should not be able to stat it in the bucket2 mount directory
	_, err = os.Stat(path.Join(bucket2MntDir, name))
	assert.NotNil(t.T(), err)
	AssertThat(err, Error(HasSubstr("no such file or directory")))

	// And we should be able to stat it in bucket1 mount directory.
	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)

	ExpectEq(name, fi.Name())
	ExpectEq(len(contents), fi.Size())

	// And read it.
	b, err := os.ReadFile(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq(contents, string(b))

	// And overwrite it, and read it back again.
	err = os.WriteFile(path.Join(bucket1MntDir, name), []byte("burrito"), 0500)
	assert.Nil(t.T(), err)

	b, err = os.ReadFile(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq("burrito", string(b))
}

func (t *MultiBucketMountCachingTest) TestFileChangedRemotely() {
	const name = "foo"
	var fi os.FileInfo
	var err error
	bucket1MntDir := getMultiMountBucketDir(bucket1Name)
	bucket1 := uncachedBuckets[bucket1Name]

	// Create a file via the file system.
	err = os.WriteFile(path.Join(bucket1MntDir, name), []byte("taco"), 0500)
	assert.Nil(t.T(), err)

	// Overwrite the object in GCS.
	_, err = storageutil.CreateObject(
		ctx,
		bucket1,
		name,
		[]byte("burrito"))

	assert.Nil(t.T(), err)

	// Because we are caching, the file should still appear to be the local
	// version.
	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq(len("taco"), fi.Size())

	// After the TTL elapses, we should see the new version.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq(len("burrito"), fi.Size())

	// Reading should work as expected.
	b, err := os.ReadFile(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectEq("burrito", string(b))
}

func (t *MultiBucketMountCachingTest) TestDirectoryRemovedRemotely() {
	const name = "foo"
	var fi os.FileInfo
	var err error
	bucket1MntDir := getMultiMountBucketDir(bucket1Name)
	bucket1 := uncachedBuckets[bucket1Name]

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(bucket1MntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Remove the backing object in GCS.
	err = bucket1.DeleteObject(
		ctx,
		&gcs.DeleteObjectRequest{Name: name + "/"})

	assert.Nil(t.T(), err)

	// Because we are caching, the directory should still appear to exist.
	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectTrue(fi.IsDir())

	// After the TTL elapses, we should see it disappear.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	_, err = os.Stat(path.Join(bucket1MntDir, name))
	ExpectTrue(os.IsNotExist(err), "err: %v", err)
}

func (t *MultiBucketMountCachingTest) TestConflictingNames_RemoteModifier() {
	const name = "foo"
	var fi os.FileInfo
	var err error
	bucket1MntDir := getMultiMountBucketDir(bucket1Name)
	bucket1 := uncachedBuckets[bucket1Name]

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(bucket1MntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Create a file with the same name via GCS.
	_, err = storageutil.CreateObject(
		ctx,
		bucket1,
		name,
		[]byte("taco"))

	assert.Nil(t.T(), err)

	// Because the file system is caching types, it will fail to find the file
	// when statting.
	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectTrue(fi.IsDir())

	_, err = os.Stat(path.Join(bucket1MntDir, name+inode.ConflictingFileNameSuffix))
	ExpectTrue(os.IsNotExist(err), "err: %v", err)

	// After the TTL elapses, we should see both.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectTrue(fi.IsDir())

	fi, err = os.Stat(path.Join(bucket1MntDir, name+inode.ConflictingFileNameSuffix))
	assert.Nil(t.T(), err)
	ExpectFalse(fi.IsDir())
}

func (t *MultiBucketMountCachingTest) TestTypeOfNameChanges_LocalModifier() {
	const name = "test"
	var fi os.FileInfo
	var err error
	bucket1MntDir := getMultiMountBucketDir(bucket1Name)

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(bucket1MntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Delete it and recreate as a file.
	err = os.RemoveAll(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)

	err = os.WriteFile(path.Join(bucket1MntDir, name), []byte("taco"), 0400)
	assert.Nil(t.T(), err)

	// All caches should have been updated.
	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectFalse(fi.IsDir())
	ExpectEq(len("taco"), fi.Size())
}

func (t *MultiBucketMountCachingTest) TestTypeOfNameChanges_RemoteModifier() {
	const name = "foo"
	var fi os.FileInfo
	var err error
	bucket1MntDir := getMultiMountBucketDir(bucket1Name)
	bucket1 := buckets[bucket1Name]

	// Create a directory via the file system.
	err = os.Mkdir(path.Join(bucket1MntDir, name), 0700)
	assert.Nil(t.T(), err)

	// Remove the backing object in GCS, updating the bucket cache (but not the
	// file system type cache)
	err = bucket1.DeleteObject(
		ctx,
		&gcs.DeleteObjectRequest{Name: name + "/"})

	assert.Nil(t.T(), err)

	// Create a file with the same name via GCS, again updating the bucket cache.
	_, err = storageutil.CreateObject(
		ctx,
		bucket1,
		name,
		[]byte("taco"))

	assert.Nil(t.T(), err)

	// Because the file system is caching types, it will fail to find the name.
	_, err = os.Stat(path.Join(bucket1MntDir, name))
	ExpectTrue(os.IsNotExist(err), "err: %v", err)

	// After the TTL elapses, we should see it turn into a file.
	cacheClock.AdvanceTime(ttl + time.Millisecond)

	fi, err = os.Stat(path.Join(bucket1MntDir, name))
	assert.Nil(t.T(), err)
	ExpectFalse(fi.IsDir())
}
