package filedb

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Most of this needs to be rewritten now that file_db uses hashes.

func getTestPath(t *testing.T) string {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		panic(fmt.Sprintf("failed to read random bytes: %v", err))
	}
	return filepath.Join(t.TempDir(), fmt.Sprintf("%x.db", buf))
}

func getTestDb(t *testing.T) *FileDb {
	path := getTestPath(t)
	t.Logf("Test path @ %s", path)
	d, err := NewFileDb(path)
	if err != nil {
		t.Fatalf("NewFileDb(%s) failed: %v", path, err)
	}
	return d
}

func TestAddFile(t *testing.T) {
	db := getTestDb(t)
	t.Cleanup(func() {
		db.Close()
		runtime.GC()
	})
	f := makeTestFile(t, "t1")
	f.SetSize(500)
	if !assert.NoErrorf(t, db.AddFile(f), "AddFile(%+v) failed", f) {
		t.FailNow()
	}
	assert.Equalf(t, f, &File{
		id:         1,
		tags:       make([]string, 0),
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      0,
		size:       500,
	}, "AddFile didn't set ID correctly")
	fl, err := db.GetFileById(f.id)
	if err != nil {
		t.Fatalf("GetFileById failed: %v", err)
	}
	assert.Equalf(t, &File{
		id:         1,
		tags:       []string{},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      0,
		size:       500,
	}, fl, "GetFileById returned incorrect structure")
}

// TODO: Add tests for 1. Just hash, 2. Just size, 3. Hash & Size
func TestAddFiles(t *testing.T) {
	db := getTestDb(t)
	t.Cleanup(func() {
		db.Close()
		runtime.GC()
	})
	fileWithSize := makeTestFile(t, "t1")
	fileWithSize.SetSize(500)
	failed, err := db.AddFiles(fileWithSize)
	if !assert.NoErrorf(t, err, "AddFiles(%+v) failed", fileWithSize) {
		t.FailNow()
	}
	if len(failed) != 0 {
		t.Errorf("Failed to insert %d files", len(failed))
		for _, f := range failed {
			t.Errorf("Failed to add file: %+v", f)
		}
		t.FailNow()
	}
	assert.Equalf(t, fileWithSize, &File{
		id:         1,
		tags:       make([]string, 0),
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      0,
		size:       500,
	}, "AddFile didn't set ID correctly")
	fl, err := db.GetFileById(fileWithSize.id)
	if err != nil {
		t.Fatalf("GetFileById failed: %v", err)
	}
	assert.Equalf(t, &File{
		id:         1,
		tags:       []string{},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      0,
		size:       500,
	}, fl, "GetFileById returned incorrect structure")
}

func TestAddFileTag(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f := makeTestFile(t, "t1")
	assert.NoErrorf(t, f.SetStars(3), "SetStars(3) failed")
	if !assert.NoErrorf(t, f.AddTag("hello-world"), "Failed to add tag 'hello-world'") {
		t.FailNow()
	}
	if !assert.NoErrorf(t, db.AddFile(f), "Failed to add file %+v", f) {
		t.FailNow()
	}
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{"hello-world"},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      3,
	}, f, "AddFile didn't add id or otherwise changed structure")
	fl, err := db.GetFileById(f.id)
	if err != nil {
		t.Fatalf("GetFileById failed: %v", err)
	}
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{"hello-world"},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      3,
	}, fl, "AddFile didn't add id or otherwise changed structure")
}

func TestUpdateFile(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f := makeTestFile(t, "t1")
	assert.NoErrorf(t, f.SetStars(2), "SetStars(2) failed")
	assert.NoErrorf(t, f.AddTag("hello-world"), "AddTag('hello-world') failed")
	if !assert.NoErrorf(t, db.AddFile(f), "AddFile failed") {
		t.FailNow()
	}
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{"hello-world"},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      2,
	}, f, "AddFile didn't add id or otherwise changed structure")
	assert.NoErrorf(t, f.SetStars(4), "SetStars(4) failed")
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{"hello-world"},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      4,
	}, f, "SetStars didn't add id or otherwise changed structure")
	assert.NoErrorf(t, f.AddTag("test"), "AddTag('test') failed")
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{"hello-world", "test"},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      4,
	}, f, "SetStars didn't add id or otherwise changed structure")
	// Remove a tag
	f.RemoveTag("hello-world")
	f.MarkFileRead()
	assert.GreaterOrEqualf(t, time.Now().UTC().Round(time.Second), f.lastViewed, "lastViewed time not update")
	err := db.UpdateFile(f)
	if err != nil {
		t.Fatalf("UpdateFile failed: %v", err)
	}
	fl, err := db.GetFileById(f.id)
	if err != nil {
		t.Fatalf("GetFileById failed: %v", err)
	}
	assert.Equal(t, f.lastViewed, fl.lastViewed, "Time wasn't equal")
	assert.Equal(t, f, fl, "GetFileById didn't return the same file")
}

func TestRemoveFile(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f := makeTestFile(t, "t1")
	assert.NoErrorf(t, f.SetStars(3), "SetStars(3) failed")
	if !assert.NoErrorf(t, db.AddFile(f), "AddFile failed") {
		t.FailNow()
	}
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      3,
	}, f, "SetStars didn't add id or otherwise changed structure")
	assert.NoErrorf(t, db.RemoveFile(f), "RemoveFile failed")
	_, err := db.GetFileById(f.id)
	if err == nil {
		t.Errorf("GetFileById succeeded when file should be removed")
	}
}

func TestGetFileByPath(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f := makeTestFile(t, "t1")
	assert.NoErrorf(t, f.SetStars(3), "SetStars(3) failed")
	assert.NoErrorf(t, db.AddFile(f), "AddFile failed")
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      3,
	}, f, "SetStars didn't add id or otherwise changed structure")
	assert.NoErrorf(t, f.SetStars(4), "SetStars(4) failed")
	assert.NoErrorf(t, f.AddTag("test"), "AddTag(test) failed")
	f.MarkFileRead()
	assert.NoErrorf(t, db.UpdateFile(f), "UpdateFile failed")
	fl, err := db.GetFileByPath("t1")
	if err != nil {
		t.Fatalf("GetFileByPath failed: %v", err)
	}
	assert.Equal(t, f, fl, "GetFileByPath didn't return correct file")
}

func TestGetFileById(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f := makeTestFile(t, "t1")
	assert.NoErrorf(t, f.SetStars(3), "SetStars(3) failed")
	assert.NoErrorf(t, db.AddFile(f), "AddFile failed")
	assert.Equal(t, &File{
		id:         1,
		tags:       []string{},
		path:       "t1",
		lastViewed: time.Unix(0, 0),
		stars:      3,
	}, f, "SetStars didn't add id or otherwise changed structure")
	assert.NoErrorf(t, f.SetStars(4), "SetStars(4) failed")
	assert.NoErrorf(t, f.AddTag("test"), "AddTag(test) failed")
	f.MarkFileRead()
	assert.NoErrorf(t, db.UpdateFile(f), "UpdateFile failed")
	fl, err := db.GetFileById(f.id)
	if err != nil {
		t.Fatalf("GetFileById failed: %v", err)
	}
	assert.Equal(t, f, fl, "GetFileById didn't return correct file")
}

func TestSearchFile(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f1 := makeTestFile(t, "test/t1")
	f1.SetSize(200)
	f1.SetStars(5)
	if !assert.NoErrorf(t, f1.AddTag("test"), "t1.AddTag(test) failed") {
		return
	}
	f1.MarkFileRead()
	assert.NotEqual(t, time.Unix(0, 0), f1.lastViewed, "lastViewed not set")
	if !assert.NoErrorf(t, db.AddFile(f1), "AddFile failed") {
		return
	}
	f2 := makeTestFile(t, "test/t2")
	f2.SetSize(100)
	f2.SetStars(1)
	if !assert.NoErrorf(t, f2.AddTag("not_test"), "t2.AddTag(not_test) failed") {
		return
	}
	f2.MarkFileRead()
	assert.NotEqual(t, time.Unix(0, 0), f2.lastViewed, "lastViewed not set")
	if !assert.NoErrorf(t, db.AddFile(f2), "AddFile failed") {
		return
	}
	f3 := makeTestFile(t, "test/t3")
	f3.SetSize(400)
	f3.SetStars(4)
	if !assert.NoErrorf(t, f3.AddTag("test"), "t3.AddTag(test) failed") {
		return
	}
	if !assert.NoErrorf(t, f3.AddTag("not_test"), "t3.AddTag(not_test) failed") {
		return
	}
	f3.MarkFileRead()
	assert.NotEqual(t, time.Unix(0, 0), f3.lastViewed, "lastViewed not set")
	if !assert.NoErrorf(t, db.AddFile(f3), "AddFile failed") {
		return
	}
	// Get file 1
	t.Run("GetFileByPartialPath", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			Path: "t1",
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f1,
			}, fs, "Search results was unexpected")
		}
	})
	t.Run("GetFileByFullPath", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			Path: "test/t1",
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f1,
			}, fs, "Search results was unexpected")
		}
	})
	// Get file 1 & 3 by tag
	t.Run("GetFileByWhitelistTag", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			WhitelistTags: []string{"test"},
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f1, f3,
			}, fs, "Search results was unexpected")
		}
	})
	t.Run("GetFileByBlacklistTag", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			BlacklistTags: []string{"not_test"},
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f1,
			}, fs, "Search results was unexpected")
		}
	})
	// Get file 1 by tag
	t.Run("GetFileByWhitelistAndBlacklistTag", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			WhitelistTags: []string{"test"},
			BlacklistTags: []string{"not_test"},
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f1,
			}, fs, "Search results was unexpected")
		}
	})
	// Get nothing
	t.Run("GetNothing", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			Path:          "test5",
			WhitelistTags: []string{"test"},
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{}, fs, "Search results was unexpected")
		}
	})
	// Get everything
	t.Run("GetEverything", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f1,
				f2,
				f3,
			}, fs, "Search results was unexpected")
		}
	})
	// Get test 2
	t.Run("GetByPathAndTags", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			Path:          "t",
			WhitelistTags: []string{"not_test"},
			BlacklistTags: []string{"test"},
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f2,
			}, fs, "Search results was unexpected")
		}
	})
	t.Run("SearchRegex", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			PathRe: "test/t[13]",
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.ElementsMatch(t, []*File{
				f1,
				f3,
			}, fs, "Search results was unexpected")
		}
	})
	t.Run("SortBySize", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			SortBy: SortMethodSize,
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.Equal(t, []*File{
				f2,
				f1,
				f3,
			}, fs, "Search results was unexpected")
		}
	})
	t.Run("SortBySizeRev", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			SortBy:      SortMethodSize,
			SortReverse: true,
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.Equal(t, []*File{
				f3,
				f1,
				f2,
			}, fs, "Search results was unexpected")
		}
	})
	// Stars are:
	// File1: 5
	// File2: 1
	// File3: 4
	t.Run("SortByStars", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			SortBy: SortMethodStars,
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.Equal(t, []*File{
				f2,
				f3,
				f1,
			}, fs, "Search results was unexpected")
		}
	})
	t.Run("SortByStarsRev", func(t *testing.T) {
		fs, err := db.SearchFile(&SearchQuery{
			SortBy:      SortMethodStars,
			SortReverse: true,
		})
		if err != nil {
			t.Errorf("SearchFile failed: %v", err)
		} else {
			assert.Equal(t, []*File{
				f1,
				f3,
				f2,
			}, fs, "Search results was unexpected")
		}
	})
}

func TestSearchCountIndex(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	for v := range 100 {
		f := makeTestFile(t, fmt.Sprintf("%d", v))
		if !assert.NoErrorf(t, f.AddTag("test"), "f%d.AddTag(test) failed", v) {
			return
		}
		if !assert.NoErrorf(t, db.AddFile(f), "AddFile(%d) failed", v) {
			return
		}
	}
	// Do nothing, wes should get files from  0-49
	files, err := db.SearchFile(&SearchQuery{
		Index: 25,
	})
	if !assert.NoErrorf(t, err, "SearchFile failed with empty query") {
		return
	}
	if !assert.Equalf(t, 50, len(files), "Expected exactly 50 results") {
		return
	}
	for i, v := range files {
		assert.Equalf(t, &File{
			// Index starts at 1
			id:         i + 26,
			tags:       []string{"test"},
			path:       fmt.Sprintf("%d", i+25),
			stars:      0,
			size:       0,
			lastViewed: time.Unix(0, 0),
			hash:       "",
		}, v, "File %d wasn't equal", i)
	}
}

func TestHasTag(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f1 := makeTestFile(t, "test2/t3")
	assert.NoErrorf(t, f1.AddTag("test"), "f.AddTag('test') failed")
	assert.NoErrorf(t, f1.AddTag("not_test"), "f.AddTag('not_test') failed")
	assert.NoErrorf(t, db.AddFile(f1), "failed to add test file")

	f2 := makeTestFile(t, "t4")
	assert.NoErrorf(t, f2.AddTag("test"), "f.AddTag('test') failed")
	assert.NoErrorf(t, f2.AddTag("123_test"), "f.AddTag('not_test') failed")
	assert.NoErrorf(t, db.AddFile(f2), "test file shouldn't have been added")

	assert.Truef(t, db.HasTag("test"), "Expected 'test' tag")
	assert.Truef(t, db.HasTag("not_test"), "Expected 'not_test' tag")
	assert.Falsef(t, db.HasTag("t888"), "Expected 't888' tag to not exist tag")
	assert.Falsef(t, db.HasTag("t444"), "Expected 't444' tag to not exist tag")
}

func TestGetAllTags(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f1 := makeTestFile(t, "test2/t3")
	// 1: test
	assert.NoErrorf(t, f1.AddTag("test"), "f.AddTag('test') failed")
	// 2: not_test
	assert.NoErrorf(t, f1.AddTag("not_test"), "f.AddTag('not_test') failed")
	assert.NoErrorf(t, db.AddFile(f1), "failed to add test file")

	f2 := makeTestFile(t, "t4")
	assert.NoErrorf(t, f2.AddTag("test"), "f.AddTag('test') failed")
	assert.NoErrorf(t, f2.AddTag("TEST"), "f.AddTag('TEST') failed")
	assert.NoErrorf(t, f2.AddTag("123_TEST"), "f.AddTag('123_TEST') failed")
	assert.Errorf(t, db.AddFile(f2), "test file shouldn't have been added")

	tgs := db.GetAllTags()
	assert.Equal(t, map[int]string{
		1: "test", 2: "not_test",
	}, tgs, "Missing tags")
}

func TestClose(t *testing.T) {
	db := getTestDb(t)
	db.Close()
	assert.Errorf(t, db.db.Ping(), "Ping should have failed")
}

func TestRemoveTag(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	f1 := makeTestFile(t, "test2/t3")
	assert.NoErrorf(t, f1.AddTag("test"), "f.AddTag('test') failed")
	assert.NoErrorf(t, f1.AddTag("not_test"), "f.AddTag('not_test') failed")
	assert.NoErrorf(t, db.AddFile(f1), "failed to add test file")

	f2 := makeTestFile(t, "t4")
	assert.NoErrorf(t, f2.AddTag("not_test"), "f.AddTag('not_test') failed")
	assert.NoErrorf(t, db.AddFile(f2), "test file shouldn't have been added")

	assert.NoErrorf(t, db.RemoveTag("not_test"), "Failed to remove tag 'test'")
	assert.Equal(t, db.GetAllTags(), map[int]string{1: "test"}, "Tags don't match")
	fl1, err := db.GetFileById(f1.id)
	if err != nil {
		t.Fatalf("GetFileById(%d) failed: %v", f1.id, err)
	}
	assert.ElementsMatch(t, fl1.tags, []string{"test"}, "File 1 tags incorrect")
	fl2, err := db.GetFileById(f2.id)
	if err != nil {
		t.Fatalf("GetFileById(%d) failed: %v", f2.id, err)
	}
	assert.ElementsMatch(t, fl2.tags, []string{}, "File 2 tags incorrect")
}

func TestFileDbAddTag(t *testing.T) {
	db := getTestDb(t)
	defer db.Close()
	assert.Equalf(t, db.GetAllTags(), map[int]string{}, "expected no tags")
	id, err := db.AddTag("tag1")
	if !assert.NoErrorf(t, err, "Failed to add tag 'tag1'") {
		return
	}
	assert.Equalf(t, id, 1, "Tag ID should have been 1")
	assert.Equalf(t, db.GetAllTags(), map[int]string{1: "tag1"}, "expected one tag")
	id2, err := db.AddTag("Tag2")
	if !assert.NoErrorf(t, err, "Failed to add tag 'tag2'") {
		return
	}
	assert.Equalf(t, id2, 2, "Tag ID should have been 2")
	assert.Equalf(t, db.GetAllTags(), map[int]string{1: "tag1", 2: "tag2"}, "expected two tags")
	// Insert into tag again
	id3, err := db.AddTag("TaG2")
	assert.Errorf(t, err, "Added 'tag2' again?, id=%d", id3)
	assert.Equalf(t, db.GetAllTags(), map[int]string{1: "tag1", 2: "tag2"}, "expected two tags")
}

// Test race conditions
