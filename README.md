## go-fitz
[![Build Status](https://github.com/gen2brain/go-fitz/actions/workflows/test.yml/badge.svg)](https://github.com/gen2brain/go-fitz/actions)
[![GoDoc](https://godoc.org/github.com/gen2brain/go-fitz?status.svg)](https://godoc.org/github.com/gen2brain/go-fitz)
[![Go Report Card](https://goreportcard.com/badge/github.com/gen2brain/go-fitz?branch=master)](https://goreportcard.com/report/github.com/gen2brain/go-fitz)

Go wrapper for [MuPDF](http://mupdf.com/) fitz library that can extract pages from PDF, EPUB, MOBI, DOCX, XLSX and PPTX documents as IMG, TXT, HTML or SVG.

### Build tags

* `extlib` - use external MuPDF library
* `static` - build with static external MuPDF library (used with `extlib`)
* `pkgconfig` - enable pkg-config (used with `extlib`)
* `musl` - use musl compiled library
* `nocgo` - experimental [purego](https://github.com/ebitengine/purego) implementation (can also be used with `CGO_ENABLED=0`)

### Custom MuPDF vendoring

This fork vendors a custom MuPDF build (headers + static libs).
If you update the MuPDF source, re-sync the vendored files with the steps below.

#### Step-by-step: rebuild MuPDF and copy into go-fitz

Use the helper script to rebuild MuPDF and copy the correct OS/arch libs:

```bash
cd /path/to/go-fitz
scripts/vendor_mupdf_local.sh --mupdf-root /path/to/mupdf
```

What it does:
* `make clean` and `make build=release` in MuPDF
* Copies `libmupdf.a` and `libmupdf-third.a` into `libs/` using the current OS/arch

To also run the full vendoring script afterward (headers + all bundled libs):

```bash
cd /path/to/go-fitz
scripts/vendor_mupdf_local.sh --mupdf-root /path/to/mupdf --full
```

You can still call the original script directly if needed:

```bash
cd /path/to/go-fitz
scripts/vendor_mupdf.sh --mupdf-root /path/to/mupdf --build release
```

Note: `vendor_mupdf.sh` only copies headers/libs from an existing MuPDF build.
If you need to rebuild MuPDF first, use `vendor_mupdf_local.sh`.

#### Verify vendored libs (hash check)

If go-fitz still appears to use old MuPDF behavior, verify that the vendored
libs match the MuPDF build output. The hashes must match exactly.

```bash
sha256sum /path/to/mupdf/build/release/libmupdf.a \\
          /path/to/go-fitz/libs/libmupdf_$(uname | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').a

sha256sum /path/to/mupdf/build/release/libmupdf-third.a \\
          /path/to/go-fitz/libs/libmupdfthird_$(uname | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').a
```

If the hashes do not match, copy the correct libs manually (example for Linux/arm64):

```bash
cp /path/to/mupdf/build/release/libmupdf.a /path/to/go-fitz/libs/libmupdf_linux_arm64.a
cp /path/to/mupdf/build/release/libmupdf-third.a /path/to/go-fitz/libs/libmupdfthird_linux_arm64.a
```

#### Verified recovery sequence (steps 1, 3, 4, 5)

Use this exact sequence when go-fitz SVG output still looks stale:

```bash
# 1) Rebuild MuPDF
cd /path/to/mupdf
scripts/compile.sh --clean -b release -t default

# 3) Copy MuPDF libs into go-fitz for current GOARCH
ARCH=$(go env GOARCH)
cp /path/to/mupdf/build/release/libmupdf.a /path/to/go-fitz/libs/libmupdf_linux_${ARCH}.a
cp /path/to/mupdf/build/release/libmupdf-third.a /path/to/go-fitz/libs/libmupdfthird_linux_${ARCH}.a

# 4) Verify copied libs are the new ones
strings /path/to/go-fitz/libs/libmupdf_linux_${ARCH}.a | grep -m1 "data-field-key"
sha256sum /path/to/mupdf/build/release/libmupdf.a /path/to/go-fitz/libs/libmupdf_linux_${ARCH}.a
sha256sum /path/to/mupdf/build/release/libmupdf-third.a /path/to/go-fitz/libs/libmupdfthird_linux_${ARCH}.a

# 5) Rebuild app (and restart your service/process)
cd /path/to/your/app
go clean -cache
go build ./...
```


Common fix (arm64 host):

```bash
cd /path/to/mupdf
make clean
make build=release

sudo cp /path/to/mupdf/build/release/libmupdf.a /path/to/go-fitz/libs/libmupdf_linux_arm64.a
sudo cp /path/to/mupdf/build/release/libmupdf-third.a /path/to/go-fitz/libs/libmupdfthird_linux_arm64.a

sha256sum /path/to/mupdf/build/release/libmupdf.a /path/to/go-fitz/libs/libmupdf_linux_arm64.a
sha256sum /path/to/mupdf/build/release/libmupdf-third.a /path/to/go-fitz/libs/libmupdfthird_linux_arm64.a
```

(Optional) Verify go-fitz still builds/tests:

```bash
cd /path/to/go-fitz
CGO_ENABLED=1 go test ./...
```

Example (this environment):

```bash
/home/ubuntu/go-fitz/scripts/vendor_mupdf_local.sh --mupdf-root /home/ubuntu/mupdf
```

#### Generate SVG directly with MuPDF (mutool)

After building MuPDF, you can generate SVG directly with `mutool`:

```bash
/home/ubuntu/mupdf/build/release/mutool draw -D -F svg -o /tmp/delme.svg /path/to/file.pdf
```

Useful checks (confirm SVG structure):

```bash
rg "<use data-text=" /tmp/delme.svg | head -n 5
rg "<text " /tmp/delme.svg | head -n 5
```

### Notes

The bundled libraries are built without CJK fonts, if you need them you must use the external library.

Calling e.g. Image() or Text() methods concurrently for the same document is not supported.

Purego implementation requires `libffi` and `libmupdf` shared libraries on runtime.
You must set `fitz.FzVersion` in your code or set `FZ_VERSION` environment variable to exact version of the shared library. 
    
### Example
```go
package main

import (
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/gen2brain/go-fitz"
)

func main() {
	doc, err := fitz.New("test.pdf")
	if err != nil {
		panic(err)
	}

	defer doc.Close()

	tmpDir, err := os.MkdirTemp(os.TempDir(), "fitz")
	if err != nil {
		panic(err)
	}

	// Extract pages as images
	for n := 0; n < doc.NumPage(); n++ {
		img, err := doc.Image(n)
		if err != nil {
			panic(err)
		}

		f, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf("test%03d.jpg", n)))
		if err != nil {
			panic(err)
		}

		err = jpeg.Encode(f, img, &jpeg.Options{jpeg.DefaultQuality})
		if err != nil {
			panic(err)
		}

		f.Close()
	}
}
```
