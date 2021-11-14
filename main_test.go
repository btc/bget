package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAndExploreHTTPPartialRequests(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		URL       string
		SizeBytes string // is a string for convenience-sake
		ETag      string
		Server    string
	}{
		{
			"https://gist.githubusercontent.com/khaykov/a6105154becce4c0530da38e723c2330/raw/41ab415ac41c93a198f7da5b47d604956157c5c3/gistfile1.txt",
			"1048575",
			`"ab7615045a59265831ab3227b5668a064c57740844d4a8fde73d0bb169993926"`,
			"", // github
		},
		{
			"https://nodejs.org/dist/v16.13.0/node-v16.13.0.tar.gz",
			"63735070",
			`"6177ee31-3cc851e"`,
			"cloudflare",
		},
		{
			"https://github.com/ipfs/ipfs-desktop/releases/download/v0.17.0/IPFS-Desktop-0.17.0.dmg",
			"125152781",
			`"f2f6f1e18404f26be370bec1db7ee970"`,
			"AmazonS3",
		},
		{
			"https://tailsde.freedif.org/tails/stable/tails-amd64-4.24/tails-amd64-4.24.img",
			"1194328064",
			"",
			"Apache/2.4.38 (Debian)",
		},
	} {
		t.Run(testCase.Server, func(t *testing.T) {
			ctx := context.Background()

			head, err := http.Head(testCase.URL)
			require.NoError(t, err)

			acceptRanges := head.Header.Get("Accept-Ranges")
			require.Equal(t, "bytes", acceptRanges)

			contentLength := head.Header.Get("Content-Length")
			require.Equal(t, testCase.SizeBytes, contentLength)

			require.Equal(t, testCase.Server, head.Header.Get("Server"))

			etag := head.Header.Get("ETag")
			resourceHasETag := etag != ""
			require.Equal(t, testCase.ETag, etag)

			req, err := newRangeRequest(ctx, testCase.URL, etag, 256, 0)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			buf, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Len(t, buf, 256)

			require.Equal(t, http.StatusPartialContent, resp.StatusCode)
			require.NotEmpty(t, resp.Header.Get("Content-Length"))
			require.NotEmpty(t, resp.Header.Get("Content-Range"))
			require.Equal(t, testCase.SizeBytes, strings.Split(resp.Header.Get("Content-Range"), "/")[1])
			if resourceHasETag {
				require.NotEmpty(t, resp.Header.Get("ETag"))
			}

			t.Run("Out Of Range", func(t *testing.T) {
				req, err := newRangeRequest(ctx, testCase.URL, etag, 100000000000, 100000000001)
				require.NoError(t, err)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusRequestedRangeNotSatisfiable, resp.StatusCode)
			})
		})
	}
}

func TestGet(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		Skip       bool
		VerifyEtag bool
		URL        string
		SizeBytes  string // is a string for convenience-sake
		HeadETag   string
		GetETag    string
		Filename   string
	}{
		{
			false,
			false,
			"https://gist.githubusercontent.com/khaykov/a6105154becce4c0530da38e723c2330/raw/41ab415ac41c93a198f7da5b47d604956157c5c3/gistfile1.txt",
			"1048575",
			`"ab7615045a59265831ab3227b5668a064c57740844d4a8fde73d0bb169993926"`,
			`W/"9d0331230ac405a0199ac0526380562147f9e0d7c8f8293c6dfc8a5299dab51b"`,
			"gistfile1.txt",
		},
		{
			true,
			false,
			"https://golang.org/dl/go1.17.3.src.tar.gz",
			"22183309",
			`"af9697"`,
			`"af9697"`,
			"go1.17.3.src.tar.gz",
		},
		{
			false,
			false,
			"https://file-examples-com.github.io/uploads/2017/10/file-example_PDF_1MB.pdf",
			"1042157",
			`"5f074cdf-fe6ed"`,
			`"5f074cdf-fe6ed"`,
			"file-example_PDF_1MB.pdf",
		},
	} {
		t.Run(testCase.Filename, func(t *testing.T) {
			if testCase.Skip {
				t.SkipNow()
			}
			head, err := http.Head(testCase.URL)
			require.NoError(t, err)

			headEtag := head.Header.Get("ETag")

			require.Equal(t, testCase.HeadETag, headEtag)

			resp, err := http.Get(testCase.URL)
			require.NoError(t, err)

			bufFromPlainGET, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)

			err = os.MkdirAll("testdata", 0755)
			require.NoError(t, err)
			err = ioutil.WriteFile("testdata/"+testCase.Filename, bufFromPlainGET, 0644)
			require.NoError(t, err)

			getETag := resp.Header.Get("ETag")
			require.Equal(t, testCase.GetETag, getETag)

			require.Equal(t, testCase.SizeBytes, fmt.Sprint(len(bufFromPlainGET)))

			path, name, err := MultiSourceGet(testCase.URL, 4, testCase.VerifyEtag)
			require.NoError(t, err)
			require.Equal(t, testCase.Filename, name)

			bufFromMultiSourceGet, err := ioutil.ReadFile(path)
			require.NoError(t, err)

			require.Equal(t, bufFromPlainGET, bufFromMultiSourceGet)
		})
	}
}

func TestFileDownload(t *testing.T) {

	t.Parallel()

	const (
		numChunks = 5
		chunkSize = 1
	)

	f, err := NewFileDownload(numChunks, chunkSize)
	require.NoError(t, err)

	select {
	case <-f.Updated():
		t.Fatal("should not have updated")
	default:
	}

	t.Run("Take", func(t *testing.T) {
		i := 0
		for {
			_, found := f.TakeUndownloadedRange()
			if !found {
				break
			}
			i += 1
		}
		require.Equal(t, numChunks, i)

		t.Run("Write", func(t *testing.T) {

			writeStringAt := func(s string, pos int64) {
				err := f.WriteAt(bytes.NewReader([]byte(s)), pos)
				require.NoError(t, err)
			}

			writeStringAt("h", 0)
			<-f.Updated() // because buffered value should be present after commit

			writeStringAt("e", 1)
			writeStringAt("l", 2)
			writeStringAt("l", 3)
			require.False(t, f.IsComplete(), "before final commit")

			writeStringAt("o", 4)
			require.True(t, f.IsComplete(), "after final commit")

			buf, err := f.Bytes()
			require.NoError(t, err)

			require.Equal(t, "hello", string(buf))
		})
	})
}
