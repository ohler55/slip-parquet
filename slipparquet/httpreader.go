// Copyright (c) 2026, Peter Ohler, All rights reserved.

package slipparquet

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// httpTimeout is how long to wait for the response headers of a range
// request. It deliberately does not bound the body read since a single column
// chunk can be large.
const httpTimeout = 30 * time.Second

// httpReader reads a parquet file over HTTP using range requests. Parquet is
// laid out so that only the footer and the column chunks actually asked for
// need to be fetched which makes reading a remote file over a network
// reasonable.
type httpReader struct {
	url    string
	client *http.Client
	auth   string
	size   int64
	pos    int64
}

// isURL returns true if the path is an HTTP or HTTPS URL.
func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// newHTTPReader returns a reader for a remote parquet file. The size of the
// file is determined and the server checked for range request support before
// returning.
func newHTTPReader(link string) (*httpReader, error) {
	r := httpReader{
		url:  link,
		auth: hubToken(link),
		client: &http.Client{
			Transport: &http.Transport{ResponseHeaderTimeout: httpTimeout},
		},
	}
	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		return nil, err
	}
	r.setAuth(req)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s on HEAD of %s", resp.Status, link)
	}
	if !strings.EqualFold(resp.Header.Get("Accept-Ranges"), "bytes") {
		return nil, fmt.Errorf("%s does not support range requests", link)
	}
	if r.size, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err != nil {
		return nil, fmt.Errorf("no content length for %s", link)
	}
	return &r, nil
}

// hubToken returns the HuggingFace access token if one is set in the
// environment and the link is to the HuggingFace hub. The token is only
// returned for hub hosts so that a token is never handed to some unrelated
// site. Redirects to the hub CDN are signed and need no token. The Go HTTP
// client drops the Authorization header on a cross domain redirect anyway.
func hubToken(link string) string {
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		return ""
	}
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "huggingface.co" || strings.HasSuffix(host, ".huggingface.co") {
		return "Bearer " + token
	}
	return ""
}

func (r *httpReader) setAuth(req *http.Request) {
	if r.auth != "" {
		req.Header.Set("Authorization", r.auth)
	}
}

// ReadAt reads len(p) bytes starting at off. It satisfies the io.ReaderAt
// contract of returning a non-nil error whenever fewer than len(p) bytes are
// read.
func (r *httpReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.size <= off {
		return 0, io.EOF
	}
	// Clamp so a read running off the end asks only for what exists.
	end := off + int64(len(p)) - 1
	if last := r.size - 1; last < end {
		end = last
	}
	req, err := http.NewRequest(http.MethodGet, r.url, nil)
	if err != nil {
		return 0, err
	}
	r.setAuth(req)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, end))
	resp, err := r.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	// A 200 means the range was ignored and the body is the whole file. Using
	// it would silently return data from the wrong offset.
	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("%s on range request of %s", resp.Status, r.url)
	}
	n, err := io.ReadFull(resp.Body, p[:end-off+1])
	if err == nil && n < len(p) {
		err = io.EOF
	}
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
}

// Read reads from the current position.
func (r *httpReader) Read(p []byte) (int, error) {
	n, err := r.ReadAt(p, r.pos)
	r.pos += int64(n)
	return n, err
}

// Seek sets the position for the next Read.
func (r *httpReader) Seek(off int64, whence int) (int64, error) {
	var pos int64
	switch whence {
	case io.SeekStart:
		pos = off
	case io.SeekCurrent:
		pos = r.pos + off
	case io.SeekEnd:
		pos = r.size + off
	default:
		return 0, fmt.Errorf("invalid whence %d", whence)
	}
	if pos < 0 {
		return 0, fmt.Errorf("negative position")
	}
	r.pos = pos
	return pos, nil
}

// Close releases the idle connections held for the file.
func (r *httpReader) Close() error {
	r.client.CloseIdleConnections()
	return nil
}
