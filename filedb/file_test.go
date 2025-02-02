package filedb

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func makeTestFile(_ *testing.T, path string) *File {
	return NewFile(path)
}

func tagCompareTest(t *testing.T, values []string) {
	// Create file
	f := NewFile("test")
	// Add values
	for i, v := range values {
		assert.NoErrorf(t, f.AddTag(v), "Failed to add tag %d => %s: %v", i, v)
	}
	assert.Equalf(t, values, f.tags, "Missed expected tags")
}

func TestAddTag(t *testing.T) {
	testCases := map[string]struct {
		Values []string
	}{
		"SingleKey": {
			[]string{
				"123", "456", "789", "10 11 12",
			},
		},
		"MultiKey": {
			[]string{
				"123", "456", "789", "10 11 12",
				"fasd", "fgdgdf", "dfssdffds", "sdafdasfsdfasdfasdfsdfa",
				"gsfgsdfg", "fsfdgfgsddfgdgdf", "dfssdffgsdfgsdfds", "sdafdsdfgsdfgdsfgasfsdfasdfasdfsdfa",
			},
		},
	}
	for k, v := range testCases {
		t.Run("AddTag"+k, func(t *testing.T) {
			tagCompareTest(t, v.Values)
		})
	}
}
func TestFileRemoveTag(t *testing.T) {
	testCases := map[string]struct {
		Values []string
		Remove []string
		After  []string
	}{
		"SingleKey": {
			Values: []string{
				"123", "456", "789", "10 11 12",
			},
			Remove: []string{"123", "789"},
			After:  []string{"456", "10 11 12"},
		},
		"Nothing": {
			Values: []string{
				"123", "456", "789", "10 11 12",
				"fasd", "fgdgdf", "dfssdffds", "sdafdasfsdfasdfasdfsdfa",
				"gsfgsdfg", "fsfdgfgsddfgdgdf", "dfssdffgsdfgsdfds", "sdafdsdfgsdfgdsfgasfsdfasdfasdfsdfa",
			},
			Remove: []string{},
			After: []string{
				"123", "456", "789", "10 11 12",
				"fasd", "fgdgdf", "dfssdffds", "sdafdasfsdfasdfasdfsdfa",
				"gsfgsdfg", "fsfdgfgsddfgdgdf", "dfssdffgsdfgsdfds", "sdafdsdfgsdfgdsfgasfsdfasdfasdfsdfa",
			},
		},
	}
	for k, v := range testCases {
		t.Run("AddTag"+k, func(t *testing.T) {
			tagCompareTest(t, v.Values)
		})
	}
}

func TestAddTagEmpty(t *testing.T) {
	f := makeTestFile(t, "test")
	assert.Error(t, f.AddTag(""), "Expected error adding empty tag")
}

func TestAddTagDupe(t *testing.T) {
	f := makeTestFile(t, "test")
	if !assert.NoError(t, f.AddTag("123"), "Failed adding tag") {
		t.FailNow()
	}
	assert.Truef(t, f.HasTag("123"), "HasTag '123' is false")
	assert.Errorf(t, f.AddTag("123"), "Expected error adding duplicate tag: Tags: %+v", f.tags)
}

func TestGetPath(t *testing.T) {
	f := File{
		path: "HELLO PATH",
	}
	assert.Equalf(t, f.path, f.GetPath(), "Expected .Path() to return correctly")
}

func TestMarkFileRead(t *testing.T) {
	expected := time.Unix(time.Now().Unix(), 0)
	f := makeTestFile(t, "test")
	f.lastViewed = expected
	assert.Equalf(t, expected, f.GetLastPlayTime(), "LastPlayTime was not expected value")
	// Ensure time is gong toi be different.
	time.Sleep(time.Microsecond * 100)
	f.MarkFileRead()
	assert.GreaterOrEqualf(t, f.GetLastPlayTime(), expected, "LastPlayTime was not past expected value")
}

func TestSetStars(t *testing.T) {
	f := makeTestFile(t, "test")
	if !assert.Equalf(t, &File{
		id:         0,
		tags:       make([]string, 0),
		path:       "test",
		lastViewed: time.Unix(0, 0),
		stars:      0,
	}, f, "Starting file was not equal") {
		t.FailNow()
	}
	for i := range uint8(6) {
		assert.NoErrorf(t, f.SetStars(i), "Failed to set stars to %d", i)
		assert.Equalf(t, i, f.GetStars(), "GetStars returned wrong value")
		assert.Equalf(t, &File{
			id:         0,
			tags:       make([]string, 0),
			path:       "test",
			lastViewed: time.Unix(0, 0),
			stars:      i,
		}, f, "Other structure values modified")
	}
	assert.Errorf(t, f.SetStars(6), "Set stars to 6 should have failed")
	assert.Equalf(t, uint8(5), f.GetStars(), "GetStars returned wrong value after attempting to set invalid value")
	assert.Errorf(t, f.SetStars(0xff), "Set stars to 6 should have failed")
	assert.Equalf(t, uint8(5), f.GetStars(), "GetStars returned wrong value after attempting to set invalid value")
}

func TestGetId(t *testing.T) {
	f := makeTestFile(t, "test")
	assert.Equal(t, 0, f.id, "GetId returned incorrect value at start")
	f.id = 5
	assert.Equal(t, 5, f.GetId(), "GetId returned incorrect value")
}

func TestGetSize(t *testing.T) {
	f := makeTestFile(t, "test")
	f.SetSize(500)
	assert.Equalf(t, int64(500), f.GetSize(), "Expected size to be 500")
}

func TestGetTags(t *testing.T) {
	f := makeTestFile(t, "test")
	assert.NoErrorf(t, f.AddTag("test"), "Failed to add tag 'test'")
	assert.NoErrorf(t, f.AddTag("123"), "Failed to add tag '123'")
	assert.NoErrorf(t, f.AddTag("456"), "Failed to add tag '456'")
	assert.NoErrorf(t, f.AddTag("789"), "Failed to add tag '789'")
	assert.ElementsMatchf(t, f.GetTags(), []string{"test", "123", "456", "789"}, "GetTags didn't return expected results")
}

func TestNewFile(t *testing.T) {
	f := NewFile("t")
	assert.Equalf(t, &File{
		id:         0,
		tags:       make([]string, 0),
		path:       "t",
		lastViewed: time.Unix(0, 0),
		stars:      0,
	}, f, "Starting file was no equal")
}

func TestNewFileWithInfo(t *testing.T) {
	const TestString string = "Hello World!"
	ExpectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(TestString)))
	// Creat test file
	file, err := os.CreateTemp(t.TempDir(), "filedb_*")
	if err != nil {
		t.Fatalf("Failed to creat temp file: %v", err)
		return
	}
	t.Logf("Path: %v", file.Name())
	defer file.Close()
	_, err = file.Write([]byte(TestString))
	if err != nil {
		t.Fatalf("Failed to write test data to test string: %v", err)
		return
	}
	f, err := NewFileWithInfo(file.Name())
	if err != nil {
		t.Fatalf("Failed to create file with hash: %v", err)
		return
	}
	assert.Equalf(t, ExpectedHash, f.GetHash(), "Invalid hash")
	assert.Equal(t, int64(len(TestString)), f.GetSize(), "Invalid size")
}
