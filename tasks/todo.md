# slip-parquet — HuggingFace browsing support

**Goal:** open a HuggingFace parquet URL directly, page through it without
materializing the whole file, and get back values that faithfully represent
what is stored (nulls, bytes, class labels).

**Scope:** Tier 1 (remote + paging + row groups) and Tier 2 (nulls, key-value
metadata, octets, statistics) from the gap analysis. Writer support and
multi-shard datasets are explicitly out of scope.

**Branch:** `feat/hf-browsing` off `develop` (after `chore/update-arrow` lands).

---

## Evidence this approach works

Verified against `stanfordnlp/imdb` before planning:

- HF serves parquet with `accept-ranges: bytes` and answers range requests `206`.
- A ~40-line `io.ReaderAt`+`io.Seeker` fed to `file.NewParquetReader` read the
  full schema, all 25 row groups, and per-column statistics in
  **3 range requests / 62 KB of a 21 MB file**.
- File key-value metadata carries the HF feature spec:
  `{"info":{"features":{"label":{"names":["neg","pos"],"_type":"ClassLabel"}}}}`

---

## Design decisions

### Settled

1. **URL detection in `parquet-open`.** A `string` beginning `http://` or
   `https://` opens a range-backed remote reader; anything else stays
   `file.OpenParquetFile`. One entry point, no new surface, existing callers
   unaffected. Same rule applies to the `:file` init keyword.
2. **`parquet.ReaderAtSeeker` is the whole contract** — `io.ReaderAt` +
   `io.Seeker` (plus `Read`). No arrow-side changes needed.
3. **Nulls map to `nil`.** Idiomatic for SLIP and already what `array.Null`
   produces today.
4. **No HF-specific decoding in Go.** Expose raw key-value metadata; let SLIP
   code parse the `huggingface` JSON. Keeps the package generic.

### Open — need a call before step 5

5. **`array.Binary` -> `slip.Octets` is a behaviour change.** In
   `testdata/primitive.parquet`, `string_col` and `date_string_col` are parquet
   `BYTE_ARRAY` with no UTF8 annotation, so they arrive as `*array.Binary` and
   today return `slip.String`. Switching unconditionally to `slip.Octets` turns
   currently-readable text into byte vectors and breaks existing tests.
   Options: (a) unconditional `Octets` + changelog note, (b) `utf8.Valid()`
   heuristic, (c) reader option `:binary-as :string|:octets` defaulting to
   current behaviour. **Recommend (c).**

6. **Null vs. false for boolean columns.** SLIP `nil` is already `false`, so a
   null boolean and a false boolean collapse. Inherent to Lisp; accept unless a
   `*parquet-null*` sentinel is wanted.

---

## Steps

### 1. HTTP range reader — DONE
New `httpreader.go`: `httpReaderAt` implementing `ReadAt`/`Read`/`Seek`.
HEAD to resolve redirects (HF redirects to a CDN host) and read
`Content-Length`; refuse if `Accept-Ranges: bytes` is absent. Configurable
timeout. Sends `Authorization: Bearer $HF_TOKEN` when that env var is set, so
gated datasets work.

*Verified:* 22 tests in `httpreader_test.go`, 100% of `newHTTPReader` and
92.6% of `ReadAt` covered. Includes serving a real fixture over HTTP and
diffing every value against the same file read from disk, and a check that
reading metadata alone fetches under half the file. Live test against imdb
behind `-tags hflive` passes (25000 rows, 25 row groups).

### 2. Wire remote into `parquet-open` — DONE
Split `initReader` into local and remote paths on URL prefix. Ensure
`:close` releases the HTTP reader (implement `io.Closer`).

*Verified:* 11 SLIP-level tests in `open_remote_test.go`. `:row-count`,
`:column-count`, `:created-by`, `:version`, `:column` and `:rows` all return
byte-identical results from a served fixture and the same file on disk.
`make-instance :file <url>` works, `:filepath` holds the URL, double close is
safe. Errors (404, no range support, unreachable, non-parquet body, missing
local file) all raise `file-error`. Live test confirms 25000 rows, 2 columns
and fields `("text" "label")` from imdb.

Note: `initReader` now takes scope and depth and raises `slip.FilePanic`
instead of a raw `panic(err)`, so a missing *local* file also became a
`file-error`. No arrow change was needed for close: `file.Reader.Close` calls
`Close` on the source when it implements `io.Closer`.

### 3. Row-group access — DONE
`:row-group-count` -> fixnum. `:row-group-info n` -> assoc list of rows,
byte size, compressed size. Standalone `parquet-row-group-count`.

*Verified:* `parquet-row-group-count` / `:row-group-count` and
`parquet-row-group-info` / `:row-group-info` added. 6 tests including one that
asserts the per-group row counts sum to `:row-count` across three fixtures,
which is the invariant paging depends on. Out-of-range and non-fixnum indexes
raise. Live: imdb reports 25 groups, group 0 has 1000 rows.

### 4. Paging and early exit
Add `:offset` / `:limit` to `:rows` and `:each-row` (and their function forms).
Use row-group metadata to skip whole groups below the offset rather than
decoding and discarding. Honour a non-nil callback return from `:each-row` as
"stop".

*Verify:* offset/limit slices match the equivalent full-read slice; reading
`:limit 10` from imdb issues no more range requests than reading row group 0
alone (assert via a counting reader); `:each-row` returning non-nil stops early
and still closes.

### 5. Null handling
`IsNull(i)` guard at the top of `ArrayValue`. New fixture with nulls in int64,
double, string, bool, and list columns — none of the seven current fixtures has
a single null, which is why this went unnoticed.

*Verify:* `[7, 0, NULL]` -> `7, 0, nil` (today it is `7, 0, 0`);
`["a", NULL]` -> `"a", nil` (today `"a", ""`). Regression test per type.

### 6. Key-value metadata
`:metadata` -> assoc list from `MetaData().KeyValueMetadata()`.
Standalone `parquet-metadata`.

*Verify:* imdb returns keys `huggingface` and `ARROW:schema`, and the
`huggingface` value parses to the feature spec containing `["neg","pos"]`.

### 7. Column statistics
`:statistics &optional column-id` -> per row-group min/max/null-count/
distinct-count, nil where absent.

*Verify:* imdb row group 0 `text` column reports non-nil min/max;
`primitive.parquet` reports nil (confirmed to carry no statistics).

### 8. Binary as octets
Per decision 5. Covers both the top-level `*array.Binary` case and the
`[]byte` case in `JSONToLisp` — HF `Image`/`Audio` features are
`struct<bytes: binary, path: string>` and reach SLIP through the JSON path,
where raw bytes currently become a `slip.String`
(confirmed: `{"bytes" . "\x89PNG\r"}`).

*Verify:* PNG magic round-trips byte-for-byte; nested struct case covered.

### 9. Docs
README section on remote/HF usage with a worked example; `:doc` strings for
every new method; CHANGELOG entry.

*Verify:* `make test` (lint + tests) green; example in README runs as written.

---

## Out of scope

Writer support (`notes.md` "next"); multi-shard datasets as one logical table;
dictionary-column fast path (correct today, just slow); explicit
Decimal32/64/128/256 cases; `context` plumbing for cancellation.
