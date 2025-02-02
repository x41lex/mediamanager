package filedb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidFile(t *testing.T) {
	okFile := &File{
		id: 1,
	}
	badFile := &File{
		id: 0,
	}
	assert.NoErrorf(t, isValidFile(okFile), "File shouldn't have returned a file")
	assert.Errorf(t, isValidFile(badFile), "File should have returned a file")
}
