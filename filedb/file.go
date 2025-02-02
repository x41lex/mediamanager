package filedb

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type File struct {
	id         int
	tags       []string
	path       string
	lastViewed time.Time
	stars      uint8 // Out of 5
	size       int64
	hash       string
}

// Adds a tag to this file, if the tag doesn't exist it will be created
// The tag cannot be empty
func (f *File) AddTag(tag string) error {
	if tag == "" {
		return errors.New("cannot use empty tag")
	}
	if f.HasTag(tag) {
		return errors.New("cannot add duplicate tag")
	}
	f.tags = append(f.tags, tag)
	return nil
}

// Remove a tag, if the tag is not found nothing happens
func (f *File) RemoveTag(tag string) {
	for i, v := range f.tags {
		if v == tag {
			f.tags = append(f.tags[:i], f.tags[i+1:]...)
			return
		}
	}
}

func (f *File) SetSize(size int64) {
	f.size = size
}

func (f *File) GetSize() int64 {
	return f.size
}

// Checks if this file has a given tag
func (f *File) HasTag(key string) bool {
	for _, v := range f.tags {
		if v == key {
			return true
		}
	}
	return false
}

func (f *File) GetTags() []string {
	return f.tags
}

// Gets the path of the file
func (f *File) GetPath() string {
	return f.path
}

func (f *File) MarkFileRead() {
	// Theres a better way to do this... right?
	f.lastViewed = time.Unix(time.Now().Unix(), 0)
}

func (f *File) GetLastPlayTime() time.Time {
	return f.lastViewed
}

func (f *File) GetStars() uint8 {
	return f.stars
}

func (f *File) SetStars(v uint8) error {
	if v > 5 {
		return errors.New("max star value is 5")
	}
	f.stars = v
	return nil
}

func (f *File) GetId() int {
	return f.id
}

func (f *File) GetHash() string {
	return f.hash
}

func NewFile(path string) *File {
	return &File{
		path:       path,
		tags:       make([]string, 0),
		id:         0,
		lastViewed: time.Unix(0, 0),
		stars:      0,
		size:       0,
		hash:       "",
	}
}

func NewFileWithInfo(path string) (*File, error) {
	// Open file
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for hashing: %v", err)
	}
	defer f.Close()
	// Get hash & size
	h, sz, err := getFileInfo(f)
	if err != nil {
		return nil, fmt.Errorf("failed to create file hash: %v", err)
	}
	file := NewFile(path)
	file.hash = fmt.Sprintf("%x", h)
	file.SetSize(sz)
	return file, nil
}
