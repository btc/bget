package main

import (
	"context"
	"errors"
	"fmt"
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
				// Is deadlock possible?
				//
				// Proof of Liveness:
				//
				// Premise: Suppose [1] all workers are sleeping in this select
				// block AND [2] the download is not complete.
				//
				// [3] Then, if the download is not complete, there exist
				// ranges that are either undownloaded or downloading.
				//
				// [4] Since all workers are here, then there are no
				// undownloaded ranges. i.e. None were 'found'.
				//
				// [5] Then, there must be at least one 'downloading' range.
				//
				// [6] If there is a 'downloading' range, then there must
				// be a worker who owns that offset and hasn't returned it.
				//
				// [7] But if all workers are here, then no workers are holding
				// offsets. So, no ranges are downloading.
				// (assuming downloadRange respects its public contract)
				//
				// [8] So, no ranges are 'undownloaded' (4) and no ranges are
				// 'downloading' (7). This contradicts the premise (2) that
				// the download is not complete.
				//
				// To the contrary, these statements demonstrate that if all
				// workers are sleeping in this block, then the download must
				// be complete.
				select {
				case <-ctx.Done():
					return nil
				case <-f.Updated():
					continue
				}
			}
			ctx, _ := context.WithTimeout(ctx, requestTimeout)
			err := downloadRange(ctx, f, url, etag, offset)
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

// Downloads a range of the file.
//
// On error, the range is marked as undownloaded.
// On success, the range is marked as downloaded.
func downloadRange(ctx context.Context, f *FileDownload, url, etag string, offset int64) (err error) {

	defer func() {
		if err != nil {
			f.ReturnUndownloadedRange(offset)
		}
	}()

	req, err := newRangeRequest(ctx, url, etag, f.ChunkSize(), offset)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return errors.New("unexpected status code: " + resp.Status)
	}

	if err := f.WriteAt(resp.Body, offset); err != nil {
		return err
	}
	return nil
}

func newRangeRequest(ctx context.Context, url, etag string, size, offset int64) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	end := offset + size - 1
	rangeValue := fmt.Sprintf("bytes=%d-%d", offset, end)
	req.Header.Set("Range", rangeValue)
	return req, nil

}
