package storage

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathTransform(t *testing.T) {
	key := "testkey"
	path := DefaultPathTransformFunc(key)

	assert.Equal(t, "913a73b565/c8e2c8ed94/497580f619/397709b8b6", path.pathName)
	assert.Equal(t, "221b368d7f5f597867f525971f28ff75", path.fileName)
}

func TestStorage(t *testing.T) {
	root := "root"
	s := NewStorage(root, DefaultPathTransformFunc)

	key := "testkey"
	data := []byte("random data")

	n, err := s.Write(key, bytes.NewReader(data))

	assert.Nil(t, err)
	assert.Equal(t, len(data), int(n))

	r, err := s.Read(key)

	assert.Nil(t, err)

	b, _ := io.ReadAll(r)

	assert.Equal(t, data, b)

	assert.Equal(t, true, s.Exists(key))

	assert.Nil(t, s.Delete(key))

	assert.Nil(t, s.Reset())
}
