package storage

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type PathKey struct {
	pathName string
	fileName string
}

func (p PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", p.pathName, p.fileName)
}

func (p PathKey) FirstPath() string {
	paths := strings.Split(p.pathName, "/")
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

type PathTransformFunc func(string) PathKey

func DefaultPathTransformFunc(key string) PathKey {
	digest := sha1.Sum([]byte(key))
	digestStr := hex.EncodeToString(digest[:])

	blockSize := 10
	var paths []string
	for i := 0; i < len(digestStr); i += blockSize {
		end := i + blockSize
		if end > len(digestStr) {
			end = len(digestStr)
		}
		paths = append(paths, digestStr[i:end])
	}

	fileNameBytes := md5.Sum([]byte(key))
	fileName := hex.EncodeToString(fileNameBytes[:])

	return PathKey{
		pathName: strings.Join(paths, "/"),
		fileName: fileName,
	}
}

type Storage struct {
	root              string
	pathTransformFunc PathTransformFunc
}

func NewStorage(root string, pathTransform PathTransformFunc) *Storage {
	return &Storage{
		root:              root,
		pathTransformFunc: pathTransform,
	}
}

func (s *Storage) Read(key string) (int64, io.Reader, error) {
	n, f, err := s.readStream(key)
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()

	buff := new(bytes.Buffer)
	_, err = io.Copy(buff, f)
	if err != nil {
		return 0, nil, err
	}

	return n, buff, nil
}

func (s *Storage) Write(key string, r io.Reader) (int64, error) {
	return s.writeStream(key, r)
}

func (s *Storage) Exists(key string) bool {
	pathKey := s.pathTransformFunc(key)
	filePath := fmt.Sprintf("%s/%s", s.root, pathKey.FullPath())

	_, err := os.Stat(filePath)
	return !errors.Is(err, os.ErrNotExist)
}

func (s *Storage) Delete(key string) error {
	pathKey := s.pathTransformFunc(key)
	firstPath := fmt.Sprintf("%s/%s", s.root, pathKey.FirstPath())

	err := os.RemoveAll(firstPath)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) Reset() error {
	return os.RemoveAll(s.root)
}

func (s *Storage) readStream(key string) (int64, io.ReadCloser, error) {
	pathKey := s.pathTransformFunc(key)
	filePath := fmt.Sprintf("%s/%s", s.root, pathKey.FullPath())

	file, err := os.Open(filePath)
	if err != nil {
		return 0, nil, err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}

	return fileInfo.Size(), file, nil
}

func (s *Storage) writeStream(key string, r io.Reader) (int64, error) {
	pathKey := s.pathTransformFunc(key)
	path := fmt.Sprintf("%s/%s", s.root, pathKey.pathName)

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return 0, err
	}

	filePath := fmt.Sprintf("%s/%s", s.root, pathKey.FullPath())

	f, err := os.Create(filePath)
	if err != nil {
		return 0, nil
	}

	n, err := io.Copy(f, r)
	if err != nil {
		return 0, err
	}

	return n, nil
}
