package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
)

// TODO: relevant? https://httpwg.org/specs/rfc7230.html#chunked.encoding

func main() {
	if err := run(); err != nil {

		log.Fatal(err)
	}
}

func run() error {

	url := flag.String("url", "https://nodejs.org/dist/v16.13.0/node-v16.13.0.tar.gz", "a URL to download")
	numWorkers := flag.Uint("workers", 4, "number of workers")
	flag.Parse()

	_, err := MultiSourceGet(*numWorkers, urls)
	if err != nil {
		return err
	}

	return nil
}

func MultiSourceGet(url string, numWorkers uint) ([]byte, error) {

	resp, err := http.Head(url)
	if err != nil {
		return nil, err
	}
	etag := resp.Header.Get("ETag")
	contentLength := resp.ContentLength

	if contentLength == -1 {
		// TODO(btc): may need to address this
		return nil, fmt.Errorf("content length is unknown")
	}

	fmt.Println(resp.Header)
	fmt.Println(etag)

	ctx, cancel := context.WithCancel(context.Background())

	for i := uint(0); i < numWorkers; i++ {
		go func(workerId uint) {
			// TODO when should a worker terminate?

			offset := 0
			for {

				select {
				case <-ctx.Done():
					return
				default:

					start, end := computeByteRange(workerId, offset, contentLength)

					req, err := newWorkerRequest(ctx, url, etag, start, end)
					if err != nil {
						// TODO: treat this as fatal?
						log.Fatal(err)
					}
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
					}

					offset += 1
				}
			}
		}(i)
	}

	data := make([]byte, contentLength)
	for {
		select {}
	}
	cancel()

	return nil, nil
}

// TODO testme
func computeByteRange(workerId uint, offset int, contentLength int64) (int, int) {
	// FIXME: pass as flag, if helpful
	const chunkSizeBytes = 1000000 // TODO express as power of 2 / bit shift
	return 0, 0
}

func newWorkerRequest(ctx context.Context, url string, etag string, start, end int) (*http.Request, error) {

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err // TODO: what can cause this error?
	}

	// "If-Match"
	//
	// For GET and HEAD methods, used in combination with a
	// Range header, it can guarantee that the new ranges
	// requested come from the same resource as the previous
	// one. If it doesn't match, then a 416 (Range Not
	// Satisfiable) response is returned."
	//
	// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/If-Match
	//
	req.Header.Set("If-Match", etag)

	rangeValue := fmt.Sprintf("bytes=%d-%d", start, end)
	req.Header.Set("Range", rangeValue)

	return req, nil
}

/*
	1. accept the filename
	2. delegate to workers
	3. each worker writes the chunk to a mutex-protected data structure
	4. a goroutine monitors the status of the data structure to see if it's done
	5. if it's done, abort the workers.

	Q: how do we know the file size? when is this discovered?
		Q: is it valid to check the Content-Length header?

	Q: what do the responses look like for each chunk of the file?

	Q: how do we specify the chunk size?
		Q: how do we specify the chunk we want?
		range header?

	Q: what's a good file for this? where can i find an appropriate file to download?
	A: 66MB https://nodejs.org/dist/v16.13.0/node-v16.13.0.tar.gz

	see:
	https://datatracker.ietf.org/doc/html/rfc7232#section-2.3.3
*/
