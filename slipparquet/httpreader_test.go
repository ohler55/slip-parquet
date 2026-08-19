// Copyright (c) 2026, Peter Ohler, All rights reserved.

// The httpReader is not exported so unlike the other tests in this package
// these are internal tests.
package slipparquet

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/ohler55/ojg/tt"
)

var content = []byte("0123456789abcdefghijklmnopqrstuvwxyz")

// rangeServer serves content with range request support.
func rangeServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "sample.parquet", time.Time{}, bytes.NewReader(content))
	}))
}

func TestIsURL(t *testing.T) {
	tt.Equal(t, true, isURL("http://example.com/x.parquet"))
	tt.Equal(t, true, isURL("https://example.com/x.parquet"))
	tt.Equal(t, false, isURL("testdata/primitive.parquet"))
	tt.Equal(t, false, isURL("/tmp/x.parquet"))
	tt.Equal(t, false, isURL("ftp://example.com/x.parquet"))
	tt.Equal(t, false, isURL(""))
}

func TestHTTPReaderSize(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()
	tt.Equal(t, int64(len(content)), r.size)
}

func TestHTTPReaderReadAt(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	buf := make([]byte, 10)
	n, err := r.ReadAt(buf, 0)
	tt.Nil(t, err)
	tt.Equal(t, 10, n)
	tt.Equal(t, "0123456789", string(buf))

	n, err = r.ReadAt(buf, 10)
	tt.Nil(t, err)
	tt.Equal(t, 10, n)
	tt.Equal(t, "abcdefghij", string(buf))

	// The last 8 bytes is how the parquet footer is found.
	foot := make([]byte, 8)
	n, err = r.ReadAt(foot, r.size-8)
	tt.Nil(t, err)
	tt.Equal(t, 8, n)
	tt.Equal(t, "stuvwxyz", string(foot))
}

func TestHTTPReaderReadAtWhole(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	buf := make([]byte, len(content))
	n, err := r.ReadAt(buf, 0)
	tt.Nil(t, err)
	tt.Equal(t, len(content), n)
	tt.Equal(t, string(content), string(buf))
}

func TestHTTPReaderReadAtPastEnd(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	buf := make([]byte, 4)
	n, err := r.ReadAt(buf, r.size)
	tt.Equal(t, io.EOF, err)
	tt.Equal(t, 0, n)

	n, err = r.ReadAt(buf, r.size+100)
	tt.Equal(t, io.EOF, err)
	tt.Equal(t, 0, n)
}

func TestHTTPReaderReadAtSpanningEnd(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	// Ask for more than is left. io.ReaderAt requires an error when short.
	buf := make([]byte, 10)
	n, err := r.ReadAt(buf, r.size-4)
	tt.Equal(t, io.EOF, err)
	tt.Equal(t, 4, n)
	tt.Equal(t, "wxyz", string(buf[:n]))
}

func TestHTTPReaderReadAtEmpty(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	n, err := r.ReadAt([]byte{}, 0)
	tt.Nil(t, err)
	tt.Equal(t, 0, n)
}

func TestHTTPReaderReadAtNegative(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	_, err = r.ReadAt(make([]byte, 4), -1)
	tt.NotNil(t, err)
}

func TestHTTPReaderRead(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	buf := make([]byte, 4)
	_, err = r.Read(buf)
	tt.Nil(t, err)
	tt.Equal(t, "0123", string(buf))

	// A second read picks up where the first left off.
	_, err = r.Read(buf)
	tt.Nil(t, err)
	tt.Equal(t, "4567", string(buf))
	tt.Equal(t, int64(8), r.pos)
}

func TestHTTPReaderSeek(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	pos, err := r.Seek(10, io.SeekStart)
	tt.Nil(t, err)
	tt.Equal(t, int64(10), pos)

	pos, err = r.Seek(2, io.SeekCurrent)
	tt.Nil(t, err)
	tt.Equal(t, int64(12), pos)

	pos, err = r.Seek(-8, io.SeekEnd)
	tt.Nil(t, err)
	tt.Equal(t, r.size-8, pos)

	buf := make([]byte, 8)
	_, err = r.Read(buf)
	tt.Nil(t, err)
	tt.Equal(t, "stuvwxyz", string(buf))
}

func TestHTTPReaderSeekBad(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	_, err = r.Seek(0, 99)
	tt.NotNil(t, err)

	_, err = r.Seek(-1, io.SeekStart)
	tt.NotNil(t, err)
}

func TestHTTPReaderNoRangeSupport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "36")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.NotNil(t, err)
}

func TestHTTPReaderNoContentLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	_, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.NotNil(t, err)
}

// A server going away part way through browsing has to surface as an error
// rather than a short read.
func TestHTTPReaderServerGone(t *testing.T) {
	ts := rangeServer()
	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()
	ts.Close()

	_, err = r.ReadAt(make([]byte, 4), 0)
	tt.NotNil(t, err)
}

func TestHTTPReaderNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := newHTTPReader(ts.URL + "/missing.parquet")
	tt.NotNil(t, err)
}

func TestHTTPReaderBadURL(t *testing.T) {
	_, err := newHTTPReader("http://%zz/bad.parquet")
	tt.NotNil(t, err)

	_, err = newHTTPReader("http://127.0.0.1:0/nothing.parquet")
	tt.NotNil(t, err)
}

// A server that ignores the range header returns the whole file. Using that
// body would hand back data from the wrong offset so it has to be an error.
func TestHTTPReaderRangeIgnored(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "36")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(content)
		}
	}))
	defer ts.Close()

	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()

	_, err = r.ReadAt(make([]byte, 4), 10)
	tt.NotNil(t, err)
}

func TestHTTPReaderRedirect(t *testing.T) {
	ts := rangeServer()
	defer ts.Close()

	rs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, ts.URL+"/sample.parquet", http.StatusFound)
	}))
	defer rs.Close()

	r, err := newHTTPReader(rs.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()
	tt.Equal(t, int64(len(content)), r.size)

	buf := make([]byte, 4)
	n, err := r.ReadAt(buf, 2)
	tt.Nil(t, err)
	tt.Equal(t, 4, n)
	tt.Equal(t, "2345", string(buf))
}

func TestHubToken(t *testing.T) {
	t.Setenv("HF_TOKEN", "")
	tt.Equal(t, "", hubToken("https://huggingface.co/x.parquet"))

	t.Setenv("HF_TOKEN", "secret")
	tt.Equal(t, "Bearer secret", hubToken("https://huggingface.co/x.parquet"))
	tt.Equal(t, "Bearer secret", hubToken("https://HuggingFace.co/x.parquet"))
	tt.Equal(t, "Bearer secret", hubToken("https://cdn.huggingface.co/x.parquet"))

	// The token must never be handed to an unrelated host.
	tt.Equal(t, "", hubToken("https://example.com/x.parquet"))
	tt.Equal(t, "", hubToken("https://us.aws.cdn.hf.co/x.parquet"))
	tt.Equal(t, "", hubToken("https://nothuggingface.co/x.parquet"))
	tt.Equal(t, "", hubToken("://bad"))
}

func TestHTTPReaderSendsToken(t *testing.T) {
	got := make(chan string, 4)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("Authorization")
		http.ServeContent(w, r, "sample.parquet", time.Time{}, bytes.NewReader(content))
	}))
	defer ts.Close()

	// The test server is not a hub host so no token is sent.
	t.Setenv("HF_TOKEN", "secret")
	r, err := newHTTPReader(ts.URL + "/sample.parquet")
	tt.Nil(t, err)
	defer func() { _ = r.Close() }()
	tt.Equal(t, "", <-got)

	// Force the header on to verify it is carried by every range request.
	r.auth = "Bearer secret"
	_, err = r.ReadAt(make([]byte, 4), 0)
	tt.Nil(t, err)
	tt.Equal(t, "Bearer secret", <-got)
}

// The point of the reader is to satisfy parquet.ReaderAtSeeker. Serve a real
// parquet fixture over HTTP and confirm arrow reads the same values from it as
// it does from the file on disk.
func TestHTTPReaderDrivesParquet(t *testing.T) {
	body, err := os.ReadFile("testdata/primitive.parquet")
	tt.Nil(t, err)

	var reqs int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			reqs++
		}
		http.ServeContent(w, r, "primitive.parquet", time.Time{}, bytes.NewReader(body))
	}))
	defer ts.Close()

	hr, err := newHTTPReader(ts.URL + "/primitive.parquet")
	tt.Nil(t, err)
	defer func() { _ = hr.Close() }()

	remote, err := file.NewParquetReader(hr)
	tt.Nil(t, err)
	defer func() { _ = remote.Close() }()

	local, err := file.OpenParquetFile("testdata/primitive.parquet", true)
	tt.Nil(t, err)
	defer func() { _ = local.Close() }()

	tt.Equal(t, local.NumRows(), remote.NumRows())
	tt.Equal(t, local.NumRowGroups(), remote.NumRowGroups())
	tt.Equal(t, local.MetaData().Schema.NumColumns(), remote.MetaData().Schema.NumColumns())
	tt.Equal(t, local.MetaData().GetCreatedBy(), remote.MetaData().GetCreatedBy())
	tt.Equal(t, true, 0 < reqs)

	// Values have to match column for column, row for row.
	rfr, err := pqarrow.NewFileReader(remote, pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	tt.Nil(t, err)
	lfr, err := pqarrow.NewFileReader(local, pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	tt.Nil(t, err)

	sch, err := rfr.Schema()
	tt.Nil(t, err)
	for i := 0; i < sch.NumFields(); i++ {
		rcr, err := rfr.GetColumn(context.Background(), i)
		tt.Nil(t, err)
		lcr, err := lfr.GetColumn(context.Background(), i)
		tt.Nil(t, err)

		rac, err := rcr.NextBatch(batchSize)
		tt.Nil(t, err)
		lac, err := lcr.NextBatch(batchSize)
		tt.Nil(t, err)
		tt.Equal(t, lac.Len(), rac.Len(), sch.Field(i).Name)

		for c, lchunk := range lac.Chunks() {
			for j := 0; j < lchunk.Len(); j++ {
				tt.Equal(t, ArrayValue(lchunk, j), ArrayValue(rac.Chunks()[c], j),
					sch.Field(i).Name, " row ", j)
			}
		}
	}
}

// Only the footer and the one column asked for should be fetched, never the
// whole file.
func TestHTTPReaderFetchesLessThanWholeFile(t *testing.T) {
	body, err := os.ReadFile("testdata/nested-struct.parquet")
	tt.Nil(t, err)

	var served int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		http.ServeContent(rec, r, "nested-struct.parquet", time.Time{}, bytes.NewReader(body))
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		b := rec.Body.Bytes()
		served += int64(len(b))
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	hr, err := newHTTPReader(ts.URL + "/nested-struct.parquet")
	tt.Nil(t, err)
	defer func() { _ = hr.Close() }()

	pr, err := file.NewParquetReader(hr)
	tt.Nil(t, err)
	defer func() { _ = pr.Close() }()

	// Reading metadata alone must not pull the 53KB of column data.
	tt.Equal(t, int64(1), pr.NumRows())
	tt.Equal(t, true, served < int64(len(body))/2,
		"fetched ", served, " of ", len(body), " bytes for metadata only")
}
