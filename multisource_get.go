package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

func MultiSourceGet(url string, numWorkers int, checkETag bool) (string, string, error) {

	resp, err := http.Head(url)
	if err != nil {
		return "", "", err
	}
	etag := resp.Header.Get("ETag")
	contentLength := resp.ContentLength

	fmt.Println("etag: ", etag)

	if contentLength == -1 {
		// TODO(btc): may need to address this
		return "", "", fmt.Errorf("content length is unknown")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const chunkSizeBytes = 256 * 1000

	dl, err := NewFileDownload(contentLength, chunkSizeBytes)
	if err != nil {
		return "", "", err
	}

	fmt.Println("chunks: ", dl.NumChunksUndownloaded())

	for i := 0; i < numWorkers; i++ {
		go func() {
			if err := RunWorker(ctx, dl, url, etag); err != nil {
				log.Println(err)
			}
		}()
	}

	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case <-dl.Updated():
		}
		if !dl.IsComplete() {
			continue
		}
		break
	}
	if checkETag && etag != "" {
		if err := verifyETag(etag, dl.Reader); err != nil {
			return "", "", err
		}
	}

	cancel()
	if err := dl.Close(); err != nil {
		return "", "", err
	}
	name := filename(url, resp.Header)
	return dl.Filename(), name, nil
}

func filename(url string, h http.Header) string {
	fromHeader := filenameFromHeader(h)
	if fromHeader != "" {
		return fromHeader
	}
	return filenameFromURL(url)
}

func filenameFromHeader(h http.Header) string {
	cd := h.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return ""
	}
	return params["filename"]
}

func filenameFromURL(url string) string {
	_, filename := path.Split(url)
	return filename
}

var ErrIsWeakETag = errors.New("weak etag not supported")

// TODO(btc): haven't gotten this working yet. i don't yet have a grasp of how
// to detect the hash function used to derive a particular etag. there appear
// to be a few known formats, but I haven't invested the time to identify,
// document, and implement. with love and understanding, the code in the
// function body is trash afaic.
func verifyETag(etag string, f func() io.Reader) error {

	if strings.HasPrefix(etag, "W/") {
		return ErrIsWeakETag
	}

	hashes := []hash.Hash{
		crc32.NewIEEE(),
		md5.New(),
		sha256.New(),
	}
	for _, hash := range hashes {
		r := f()
		_, err := io.Copy(hash, r)
		if err != nil {
			return err
		}

		sum := hash.Sum(nil)
		ssum := fmt.Sprintf(`"%x"`, sum)
		fmt.Println(ssum)
		if ssum == etag {
			return nil
		}
	}

	return errors.New("unable to validate data for etag: " + etag)
}

func isValidMD5(etag string, buf []byte) bool {

	var isValidMD5 bool
	hash := md5.Sum(buf)
	hashStr := fmt.Sprintf("%x", hash)

	if hashStr == etag {
		isValidMD5 = true
	}

	return isValidMD5
}
