# Changelog

All notable changes to this fork are documented here.

The format follows Keep a Changelog, and this project adheres to Semantic Versioning.

## Unreleased
### Added
- `scripts/vendor_mupdf.sh` to re-sync vendored MuPDF headers and Linux amd64 static libs from a local MuPDF checkout.
- `CHANGELOG.md` to track fork-specific changes.

### Changed
- Vendored MuPDF headers replaced with the customized MuPDF headers from the local MuPDF tree.
- Linux amd64 static libs in `libs/` replaced with custom MuPDF build outputs.
