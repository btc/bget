package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// NB: can play with this value to force retries. from btc's location, values under 200ms do the trick.
const requestTimeout = time.Second * 500

// Runs the worker until the provided context is canceled.
//
// NB: url could be a member of the file download but it is passed explicitly
// so the caller can decide to provide different URLs to different workers.
func RunWorker(ctx context.Context, f *FileDownload, url, etag string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			offset, found := f.TakeUndownloadedRange()
			if !found {
				select {
				case <-ctx.Done():
					return nil
				case <-f.Updated():
					// it's possibvle for worker to miss a notification.
					// can all workers end up asleep?
					continue
				}
			}
			ctx, _ := context.WithTimeout(ctx, requestTimeout)
			body, err := getChunk(ctx, f, url, etag, offset)
			if err != nil {
				body.Close()
				log.Println(err)
				f.ReturnUndownloadedRange(offset)
				continue
			}
			if err := f.WriteAt(body, offset); err != nil {
				body.Close()
				log.Println(err)
				f.ReturnUndownloadedRange(offset)
				continue
			}
			body.Close()
		}
	}
}

func getChunk(ctx context.Context, f *FileDownload, url, etag string, start int64) (io.ReadCloser, error) {

	req, err := newRangeRequest(ctx, url, etag, f.ChunkSize(), start)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
		/*
			// NB: this arguably wasteful intermediate buffer is used to simplify
			// the API contract and avoid having the caller be responsible for
			// Body.Close(). If profiling reveals the allocations are non-trivial,
			// then switch the approach.
			var buf bytes.Buffer
			_, err := io.Copy(&buf, resp.Body)
			if err != nil {
				return nil, err
			}
			return &buf, nil
		*/

		return resp.Body, nil
	case http.StatusRequestedRangeNotSatisfiable: // TODO?
	default: // TODO handle other errors
	}
	return nil, errors.New("unknown error")
}

func newRangeRequest(ctx context.Context, url, etag string, size int, start int64) (*http.Request, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err // TODO: what can cause this error?
	}

	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	end := start + int64(size) - 1
	rangeValue := fmt.Sprintf("bytes=%d-%d", start, end)
	req.Header.Set("Range", rangeValue)

	return req, nil
}
