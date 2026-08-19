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

### Changed
- Migrated from `github.com/apache/arrow/go/v15` v15.0.2 to `github.com/apache/arrow-go/v18` v18.7.0.
  Arrow Go moved out of the Arrow monorepo into its own repository; the old module path is no
  longer maintained. Import paths only, no API changes.
- Opening a file that can not be read now raises a _file-error_ instead of a raw error. This
  applies to local files as well as remote ones.

### Fixed
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
