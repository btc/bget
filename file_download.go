package main

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"sync"
)

const chunkSizeBytes = 256 * 1000

type FileDownload struct {
	mu           sync.Mutex
	temp         *os.File
	undownloaded map[int64]struct{}
	downloading  map[int64]struct{}
	updated      chan struct{}
}

func NewFileDownload(sizeBytes int64, chunkSizeBytes int64) (*FileDownload, error) {

	f, err := ioutil.TempFile("", "") // TODO: use etag for resumable downloads
	if err != nil {
		return nil, err
	}

	undownloaded := make(map[int64]struct{})
	for i := int64(0); i < sizeBytes; i += chunkSizeBytes {
		undownloaded[i] = struct{}{}
	}

	return &FileDownload{
		temp:         f,
		undownloaded: undownloaded,
		downloading:  make(map[int64]struct{}),
		updated:      make(chan struct{}, 1), // necessary?
	}, nil
}

func (f *FileDownload) WriteAt(r io.Reader, offset int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	defer f.notify()

	if _, ok := f.downloading[offset]; !ok {
		return errors.New("this chunk is not being downloaded")
	}

	_, err := f.temp.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f.temp, r); err != nil {
		return err
	}
	// TODO: |n| should be chunkSizeBytes unless at the end of the file

	delete(f.downloading, offset)

	return nil
}

func (f *FileDownload) IsComplete() bool {
	f.mu.Lock()
	defer f.mu.Unlock() // NB: non-zero cost
	return len(f.undownloaded) == 0 && len(f.downloading) == 0
}

func (f *FileDownload) ChunkSize() int {
	return chunkSizeBytes
}

func (f *FileDownload) ReturnUndownloadedRange(offset int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// NB: O(1)

	if _, ok := f.downloading[offset]; ok {
		delete(f.downloading, offset)
		f.undownloaded[offset] = struct{}{}
		f.notify()
	}
}

func (f *FileDownload) notify() {
	for {
		select {
		case f.updated <- struct{}{}:
			continue
		default:
			return
		}
	}
}

func (f *FileDownload) NumChunksUndownloaded() int {
	return len(f.undownloaded)
}

func (f *FileDownload) TakeUndownloadedRange() (int64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// NB: O(1)

	if len(f.undownloaded) == 0 {
		return 0, false
	}

	// take the first element, NB: order not guaranteed
	var val int64
	for k := range f.undownloaded {
		val = k
		break
	}

	delete(f.undownloaded, val)
	f.downloading[val] = struct{}{} // TODO: can set time.Time

	return val, true
}

func (f *FileDownload) Close() error {
	return f.temp.Close()
}

func (f *FileDownload) Filename() string {
	return f.temp.Name()
}

func (f *FileDownload) Updated() <-chan struct{} {
	return f.updated
}

func (f *FileDownload) Bytes() ([]byte, error) {
	_, err := f.temp.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	if _, err := io.Copy(&buf, f.temp); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
