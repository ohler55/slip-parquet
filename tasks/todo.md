# slip-parquet — HuggingFace browsing support

**Goal:** open a HuggingFace parquet URL directly, page through it without
materializing the whole file, and get back values that faithfully represent
what is stored (nulls, bytes, class labels).

**Scope:** Tier 1 (remote + paging + row groups) **complete** and Tier 2
(nulls, key-value metadata, octets, statistics) from the gap analysis. Writer support and
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

### Resolved during implementation

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

### 4. Paging and early exit — DONE
`:offset` / `:limit` added to `:rows`, `:each-row` and both function forms as
`&key` arguments. `rowGroupsFor` picks only the row groups covering the
requested range, so groups below the offset and above the limit are never read.
Rows to drop inside the first selected group are skipped with `NextBatch`
rather than converted to lisp objects. With no offset and no limit the old
whole-file `GetColumn` path is used unchanged.

**Deviation:** the plan said to treat a non-nil callback return from
`:each-row` as "stop". That was dropped — a callback ending in `print` or
`setq` returns non-nil and would silently stop after one row. `block` /
`return-from` already unwinds cleanly out of the Go loop, and the reader
belongs to the caller so nothing leaks. Tested and documented instead.

*Verified:* 13 tests. `TestRowsPagingMatchesFullRead` cross-checks all 289
offset/limit combinations from 0..16 against `subseq` of a full read. New
fixture `testdata/multi-group.parquet` (15 rows in 3 groups of 5) makes group
skipping observable. `TestPagingRemoteFetchesLess` shows `:limit 1` transfers
188 bytes where a full read takes 564. `TestEachRowEarlyExit` confirms
`return-from` stops the walk and leaves the reader usable. Live: 5 rows at
offset 10000 of the 21MB imdb file, and rows 12498-12501 return
`((0) (0) (1) (1))` — the exact neg/pos boundary of the split.

### 5. Null handling — DONE
`IsNull(i)` guard at the top of `ArrayValue`. New fixture with nulls in int64,
double, string, bool, and list columns — none of the seven current fixtures has
a single null, which is why this went unnoticed.

*Verified:* new fixture `testdata/nulls.parquet` holds 8 columns each with a
value, a null, and the natural zero value, so null and zero can be told apart.
11 SLIP level tests cover `:rows`, `:column`, `:each-row`, `:assoc` and paging.

Measured before and after on the fixture:

| column | before | after |
|---|---|---|
| i64 | `(7) (0) (0)` | `(7) (nil) (0)` |
| f64 | `(1.5) (0) (0)` | `(1.5) (nil) (0)` |
| str | `("a") ("") ("")` | `("a") (nil) ("")` |
| date | `(...) (@1970-01-01) (@1970-01-01)` | `(...) (nil) (@1970-01-01)` |

The date case was the worst of them: a null produced a real date that was
never in the file.

**Existing tests had encoded the bug.** All 17 `ArrayValue` tests in
`arrayvalue_test.go` appended a null and asserted it read back as the zero
value. Those assertions were corrected to `nil`. Booleans already expected
`nil` since lisp `nil` is false, so nothing changed there.

Decision 6 stands: a null boolean and a false boolean both read as `nil` and
that is inherent to lisp rather than something this package can fix.

### 6. Key-value metadata — DONE
`:metadata` -> assoc list from `MetaData().KeyValueMetadata()`.
Standalone `parquet-metadata`.

*Verified:* `parquet-metadata` and `:metadata` return an assoc list, or nil for
a file with no key-value metadata. 6 tests over fixtures written by three
different applications, which also settles the question of whether this is a
HuggingFace specific feature — it is not:

| fixture | keys |
|---|---|
| `binary.parquet` | `parquet.proto.descriptor`, `writer.model.name`, `parquet.proto.class` |
| `nested-maps.parquet`, `nested-list.parquet` | `org.apache.spark.sql.parquet.row.metadata` |
| `null-list.parquet` | `ARROW:schema` |
| imdb (live) | `huggingface`, `ARROW:schema` |

Live test confirms the `huggingface` value carries `"names": ["neg", "pos"]`.
Per settled decision 4 the value is returned as written and parsing is left to
lisp, so no HuggingFace knowledge is in the Go code.

### 7. Column statistics — DONE
`:statistics &optional column-id` -> per row-group min/max/null-count/
distinct-count, nil where absent.

*Verified:* `parquet-statistics` and `:statistics` take an optional column id.
With one, a list of per row group assoc lists with keys `row-group`,
`num-values`, `min`, `max`, `null-count`, `distinct-count`. Without one, every
column keyed by its leaf path. 9 tests.

`multi-group.parquet` reports min/max of 0-4, 5-9, 10-14 across its three row
groups, which shows the values really are per chunk. `nulls.parquet` reports
`null-count 1` without reading any data. `primitive.parquet` carries no
statistics at all so only `row-group` and `num-values` appear — absent
statistics are a normal case rather than an error.

Columns are identified by leaf path since that is what parquet keys statistics
by, so `nulls.parquet` exposes `lst.list.element` and `st.a`. Live: imdb has 25
entries for `label`, first group max 0 and last group min 1, confirming the
split is sorted.

### 8. Binary as octets — DONE
Per decision 5. Covers both the top-level `*array.Binary` case and the
`[]byte` case in `JSONToLisp` — HF `Image`/`Audio` features are
`struct<bytes: binary, path: string>` and reach SLIP through the JSON path,
where raw bytes currently become a `slip.String`
(confirmed: `{"bytes" . "\x89PNG\r"}`).

*Verified:* **Decision 5 resolved as option (c).** A reader option
`:binary-as :string|:octets` on `parquet-open` and the `:file` init keywords,
defaulting to `:string` which is today's behaviour. Parquet cannot tell text
from bytes for an unannotated `BYTE_ARRAY`, so the file genuinely does not
carry the answer and the caller has to supply it. Nothing existing changed.

Both paths are covered: the typed `*array.Binary` case and the `[]byte` case in
`JSONToLisp`, which is how a nested HuggingFace image feature
(`struct<bytes, path>`) reaches lisp.

Exported API stayed additive: `ArrayValue` and `JSONToLisp` keep their
signatures and behaviour, with `ArrayValueOctets` and `JSONToLispOctets` added
alongside. Both delegate to a private implementation.

10 tests. PNG magic round-trips as `slip.Octets{0x89 0x50 0x4e 0x47}` where the
string form mangles it. A null binary is still nil rather than empty octets.
Text columns are unaffected.

**Gotcha worth recording:** a flavor init keyword whose name matches an
instance variable is assigned by the flavor *before* `:init` runs and is
stripped from the `:init` arguments. `initReader` was overwriting it with nil.
`:init` now reads the instance variable back to validate it.

### 9. Docs — DONE
README section on remote/HF usage with a worked example; `:doc` strings for
every new method; CHANGELOG entry.

*Verified:* README gained a Usage section covering rows, paging, early exit,
remote files, row group info, statistics, metadata, nulls, and `:binary-as`.
Every documented form is executed by `readme_test.go` against
`multi-group.parquet` and checked against the exact output printed in the
README, so the examples cannot drift. `make test` green.

---

## Out of scope

Writer support (`notes.md` "next"); multi-shard datasets as one logical table;
dictionary-column fast path (correct today, just slow); explicit
Decimal32/64/128/256 cases; `context` plumbing for cancellation.
