# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- `parquet-open` and the `:file` init keyword now accept an HTTP or HTTPS URL. Remote files are
  read with range requests so only the parts used are fetched. The `HF_TOKEN` environment
  variable, if set, authorizes requests to huggingface.co.
- `parquet-row-group-count` and the `:row-group-count` method.
- `parquet-row-group-info` and the `:row-group-info` method.
- `:offset` and `:limit` keywords on `parquet-rows`, `parquet-each-row`, `:rows` and `:each-row`.
  Only the row groups holding the requested rows are read.
- `parquet-metadata` and the `:metadata` method returning the file key-value metadata as an
  assoc list.
- `parquet-statistics` and the `:statistics` method returning per row group column statistics
  of min, max, null count, distinct count, and value count.
- A `:binary-as` option on `parquet-open` and the reader init keywords. It takes _:string_,
  the default and previous behaviour, or _:octets_ for columns holding image, audio, or other
  binary payloads. Parquet does not distinguish text from bytes unless the writer annotated
  the column so the choice belongs to the caller.
- `ArrayValueOctets` and `JSONToLispOctets` go functions alongside the existing `ArrayValue`
  and `JSONToLisp` which are unchanged.
- A Usage section in the README covering rows, paging, remote files, row groups, statistics,
  metadata, nulls, and binary columns. Every example in it is run by the tests.
- `parquet-open` and the `:file` init keyword accept a list of files. The files are read as one
  table with the row counts summed and the rows read shard by shard, which is the shape of a
  HuggingFace dataset split. Shards must share a schema.
- `parquet-explore` writes a few rows of a reader as a markdown or TSV table. Values are cut
  short so a column of long text stays readable and only the rows shown are read, so it is
  cheap on a large or remote file. Takes `:format`, `:limit`, `:offset`, `:width`, `:null`,
  `:summary`, and `:stream`.
- A `:timeout` option on `parquet-open` and the reader init keywords giving the number of
  seconds a remote server has to answer. The default is 30, which is what was previously
  hard coded. It covers connecting and waiting for the response headers and does not bound
  the transfer of the body.
- A getting started guide, [GETTING_STARTED.md](GETTING_STARTED.md), walking through opening a
  31 shard HuggingFace dataset over HTTP, looking at it, and printing rows from the middle
  without downloading the several gigabytes it holds.

### Changed
- Migrated from `github.com/apache/arrow/go/v15` v15.0.2 to `github.com/apache/arrow-go/v18` v18.7.0.
  Arrow Go moved out of the Arrow monorepo into its own repository; the old module path is no
  longer maintained. Import paths only, no API changes.
- Opening a file that can not be read now raises a _file-error_ instead of a raw error. This
  applies to local files as well as remote ones.
- `parquet-row-group-info` and `parquet-statistics` gained a _shard_ key naming the file a row
  group came from. Row group numbers now run across the whole dataset.
- Stopping part way through `:each-row` with a _block_ and _return-from_ is now documented and
  covered by tests. The reader is left open so it can still be used afterwards.
- Reads are made under a context owned by the reader rather than `context.Background`. Closing
  a reader cancels it so a read waiting on a slow server is given up on. Remote requests carry
  the context so the cancellation reaches the network rather than stopping at the arrow layer.
- Dictionary encoded columns are read by following the index into the dictionary rather than by
  encoding each value and parsing it back. 1.4 times faster with half the allocations. Values
  are unchanged.

### Removed
- Dead `:binary-as` and `:timeout` handling in the reader `:init` method. A flavor init keyword
  that matches an instance variable is assigned before `:init` runs and never reaches it, so
  those branches could not execute. Behaviour is unchanged.

### Fixed
- Passing a non-keyword where `parquet-open` expects an option now raises a _type-error_. It
  was silently ignored along with the value following it, so a mistyped option looked like it
  had been applied.
- A `parquet-open` that fails part way through no longer leaks the file it opened. A remote
  file that could not be parsed left its connections open and a file whose arrow reader could
  not be built left the parquet reader open.
- Decimal column values keep the scale the column declares. A decimal of scale 2 holding 10.50
  read as _"10.5"_ and one holding 0.00 read as _"0"_ because the value was formatted through a
  float which drops trailing zeros. They now read as _"10.50"_ and _"0.00"_. Decimals are read
  on the typed path rather than by encoding each value and parsing it back. There was no loss
  of precision before and there is none now.
- A null column value is now _nil_ instead of the zero value of the column type. Previously a
  null integer was indistinguishable from a 0, a null string from an empty string, and a null
  date read as the epoch which is a date that was never in the file.

## [1.2.0] - 2025-09-06
### Changed
- Updated to v1.2.0 of slip.

## [1.1.0] - 2025-06-23
### Changed
- Updated to v1.1.0 of slip.

## [1.0.1] - 2025-04-27
### Changed
- Updated to v1.0.1 of slip.

## [0.9.5] - 2024-10-11
### Changed
- Updated to v0.9.5 of slip.
