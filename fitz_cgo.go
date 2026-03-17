//go:build cgo && !nocgo

package fitz

/*
#include <mupdf/fitz.h>
#include <mupdf/fitz/output-svg.h>
#include <pthread.h>
#include <stdlib.h>

const char *fz_version = FZ_VERSION;
#if defined(_WIN32)
	typedef unsigned long long store;
#else
	typedef unsigned long store;
#endif

static pthread_once_t go_fitz_locks_once = PTHREAD_ONCE_INIT;
static pthread_mutex_t go_fitz_mutexes[FZ_LOCK_MAX];

static void go_fitz_init_locks(void) {
	for (int i = 0; i < FZ_LOCK_MAX; ++i) {
		pthread_mutex_init(&go_fitz_mutexes[i], NULL);
	}
}

static void go_fitz_lock(void *user, int lock) {
	pthread_mutex_t *mutexes = (pthread_mutex_t *)user;
	pthread_mutex_lock(&mutexes[lock]);
}

static void go_fitz_unlock(void *user, int lock) {
	pthread_mutex_t *mutexes = (pthread_mutex_t *)user;
	pthread_mutex_unlock(&mutexes[lock]);
}

fz_context *new_context_threadsafe(store max_store, const char *version) {
	fz_locks_context locks;

	pthread_once(&go_fitz_locks_once, go_fitz_init_locks);
	locks.user = go_fitz_mutexes;
	locks.lock = go_fitz_lock;
	locks.unlock = go_fitz_unlock;

	return fz_new_context_imp(NULL, &locks, max_store, version);
}

fz_context *clone_context_safe(fz_context *ctx) {
	fz_try(ctx) {
		return fz_clone_context(ctx);
	}
	fz_catch(ctx) {
		return NULL;
	}
}

fz_document *open_document(fz_context *ctx, const char *filename) {
	fz_document *doc;

	fz_try(ctx) {
		doc = fz_open_document(ctx, filename);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return doc;
}

fz_document *open_document_with_stream(fz_context *ctx, const char *magic, fz_stream *stream) {
	fz_document *doc;

	fz_try(ctx) {
		doc = fz_open_document_with_stream(ctx, magic, stream);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return doc;
}

fz_page *load_page(fz_context *ctx, fz_document *doc, int number) {
	fz_page *page;

	fz_try(ctx) {
		page = fz_load_page(ctx, doc, number);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return page;
}

int run_page_contents(fz_context *ctx, fz_page *page, fz_device *dev, fz_matrix transform, fz_cookie *cookie) {
	fz_try(ctx) {
		fz_run_page_contents(ctx, page, dev, transform, cookie);
	}
	fz_catch(ctx) {
		return 0;
	}

	return 1;
}

int run_page_annots(fz_context *ctx, fz_page *page, fz_device *dev, fz_matrix transform, fz_cookie *cookie) {
	fz_try(ctx) {
		fz_run_page_annots(ctx, page, dev, transform, cookie);
	}
	fz_catch(ctx) {
		return 0;
	}

	return 1;
}

int run_page_widgets(fz_context *ctx, fz_page *page, fz_device *dev, fz_matrix transform, fz_cookie *cookie) {
	fz_try(ctx) {
		fz_run_page_widgets(ctx, page, dev, transform, cookie);
	}
	fz_catch(ctx) {
		return 0;
	}

	return 1;
}

int run_display_list_safe(fz_context *ctx, fz_display_list *list, fz_device *dev, fz_matrix transform, fz_cookie *cookie) {
	fz_try(ctx) {
		fz_run_display_list(ctx, list, dev, transform, fz_infinite_rect, cookie);
	}
	fz_catch(ctx) {
		return 0;
	}

	return 1;
}

fz_display_list *new_display_list_from_page_safe(fz_context *ctx, fz_page *page) {
	fz_display_list *list = NULL;

	fz_try(ctx) {
		list = fz_new_display_list_from_page(ctx, page);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return list;
}

fz_display_list *new_display_list_from_page_contents_safe(fz_context *ctx, fz_page *page) {
	fz_display_list *list = NULL;

	fz_try(ctx) {
		list = fz_new_display_list_from_page_contents(ctx, page);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return list;
}

fz_stext_page *new_stext_page_from_display_list_safe(fz_context *ctx, fz_display_list *list, const fz_stext_options *options) {
	fz_try(ctx) {
		return fz_new_stext_page_from_display_list(ctx, list, options);
	}
	fz_catch(ctx) {
		return NULL;
	}
}

fz_device *new_svg_device_opts(fz_context *ctx, fz_output *out, float page_width, float page_height,
	int text_format, int reuse_images, int resolution) {
	fz_svg_device_options opts;
	fz_init_svg_device_options(ctx, &opts);
	opts.text_format = text_format;
	opts.reuse_images = reuse_images;
	if (resolution > 0)
		opts.resolution = resolution;
	return fz_new_svg_device_with_options(ctx, out, page_width, page_height, &opts);
}
*/
import "C"

import (
	"html"
	"image"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"unsafe"
)

// Document represents fitz document.
type Document struct {
	ctx    *C.struct_fz_context
	data   []byte // binds data to the Document lifecycle avoiding premature GC
	doc    *C.struct_fz_document
	mtx    sync.Mutex
	stream *C.fz_stream
}

var (
	htmlBlockCloseRe = regexp.MustCompile(`(?i)</(p|div|tr|li|h[1-6])>`)
	htmlBreakRe      = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe        = regexp.MustCompile(`(?s)<[^>]+>`)
)

func htmlToText(htmlContent string) string {
	htmlContent = htmlBlockCloseRe.ReplaceAllString(htmlContent, "\n")
	htmlContent = htmlBreakRe.ReplaceAllString(htmlContent, "\n")
	htmlContent = htmlTagRe.ReplaceAllString(htmlContent, "")
	return html.UnescapeString(htmlContent)
}

type pageWorker struct {
	ctx  *C.struct_fz_context
	list *C.fz_display_list
}

func (w *pageWorker) close() {
	if w == nil {
		return
	}
	if w.list != nil {
		C.fz_drop_display_list(w.ctx, w.list)
	}
	if w.ctx != nil {
		C.fz_drop_context(w.ctx)
	}
}

// New returns new fitz document.
func New(filename string) (f *Document, err error) {
	f = &Document{}

	filename, err = filepath.Abs(filename)
	if err != nil {
		return
	}

	if _, e := os.Stat(filename); e != nil {
		err = ErrNoSuchFile
		return
	}

	f.ctx = (*C.struct_fz_context)(unsafe.Pointer(C.new_context_threadsafe(C.store(MaxStore), C.fz_version)))
	if f.ctx == nil {
		err = ErrCreateContext
		return
	}

	C.fz_register_document_handlers(f.ctx)

	cfilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cfilename))

	f.doc = C.open_document(f.ctx, cfilename)
	if f.doc == nil {
		err = ErrOpenDocument
		return
	}

	ret := C.fz_needs_password(f.ctx, f.doc)
	v := int(ret) != 0
	if v {
		err = ErrNeedsPassword
	}

	return
}

// NewFromMemory returns new fitz document from byte slice.
func NewFromMemory(b []byte) (f *Document, err error) {
	if len(b) == 0 {
		return nil, ErrEmptyBytes
	}
	f = &Document{}

	f.ctx = (*C.struct_fz_context)(unsafe.Pointer(C.new_context_threadsafe(C.store(MaxStore), C.fz_version)))
	if f.ctx == nil {
		err = ErrCreateContext
		return
	}

	C.fz_register_document_handlers(f.ctx)

	f.stream = C.fz_open_memory(f.ctx, (*C.uchar)(&b[0]), C.size_t(len(b)))
	if f.stream == nil {
		err = ErrOpenMemory
		return
	}

	magic := contentType(b)
	if magic == "" {
		err = ErrOpenMemory
		return
	}

	f.data = b

	cmagic := C.CString(magic)
	defer C.free(unsafe.Pointer(cmagic))

	f.doc = C.open_document_with_stream(f.ctx, cmagic, f.stream)
	if f.doc == nil {
		err = ErrOpenDocument
	}

	ret := C.fz_needs_password(f.ctx, f.doc)
	v := int(ret) != 0
	if v {
		err = ErrNeedsPassword
	}

	return
}

// NewFromReader returns new fitz document from io.Reader.
func NewFromReader(r io.Reader) (f *Document, err error) {
	b, e := io.ReadAll(r)
	if e != nil {
		err = e
		return
	}

	f, err = NewFromMemory(b)

	return
}

// NumPage returns total number of pages in document.
func (f *Document) numPageLocked() int {
	return int(C.fz_count_pages(f.ctx, f.doc))
}

func (f *Document) NumPage() int {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	return f.numPageLocked()
}

func (f *Document) snapshotPageWorker(pageNumber int, fullPage bool) (*pageWorker, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.numPageLocked() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}
	defer C.fz_drop_page(f.ctx, page)

	var list *C.fz_display_list
	if fullPage {
		list = C.new_display_list_from_page_safe(f.ctx, page)
	} else {
		list = C.new_display_list_from_page_contents_safe(f.ctx, page)
	}
	if list == nil {
		return nil, ErrCreateDisplayList
	}

	ctx := C.clone_context_safe(f.ctx)
	if ctx == nil {
		C.fz_drop_display_list(f.ctx, list)
		return nil, ErrCloneContext
	}

	return &pageWorker{ctx: ctx, list: list}, nil
}

// Image returns image for given page number.
func (f *Document) Image(pageNumber int) (*image.RGBA, error) {
	return f.ImageDPI(pageNumber, 300.0)
}

// ImageDPI returns image for given page number and DPI.
func (f *Document) ImageDPI(pageNumber int, dpi float64) (*image.RGBA, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.numPageLocked() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(dpi/72), C.float(dpi/72))

	var bbox C.fz_irect
	bounds = C.fz_transform_rect(bounds, ctm)
	bbox = C.fz_round_rect(bounds)

	pixmap := C.fz_new_pixmap_with_bbox(f.ctx, C.fz_device_rgb(f.ctx), bbox, nil, 1)
	if pixmap == nil {
		return nil, ErrCreatePixmap
	}

	C.fz_clear_pixmap_with_value(f.ctx, pixmap, C.int(0xff))
	defer C.fz_drop_pixmap(f.ctx, pixmap)

	device := C.fz_new_draw_device(f.ctx, ctm, pixmap)
	C.fz_enable_device_hints(f.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(f.ctx, device)

	drawMatrix := C.fz_identity
	ret := C.run_page_contents(f.ctx, page, device, drawMatrix, nil)
	if ret == 0 {
		return nil, ErrRunPageContents
	}
	ret = C.run_page_annots(f.ctx, page, device, drawMatrix, nil)
	if ret == 0 {
		return nil, ErrRunPageContents
	}
	ret = C.run_page_widgets(f.ctx, page, device, drawMatrix, nil)
	if ret == 0 {
		return nil, ErrRunPageContents
	}

	C.fz_close_device(f.ctx, device)

	pixels := C.fz_pixmap_samples(f.ctx, pixmap)
	if pixels == nil {
		return nil, ErrPixmapSamples
	}

	img := image.NewRGBA(image.Rect(int(bbox.x0), int(bbox.y0), int(bbox.x1), int(bbox.y1)))
	copy(img.Pix, C.GoBytes(unsafe.Pointer(pixels), C.int(4*bbox.x1*bbox.y1)))

	return img, nil
}

// ImagePNG returns image for given page number as PNG bytes.
func (f *Document) ImagePNG(pageNumber int, dpi float64) ([]byte, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.numPageLocked() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(dpi/72), C.float(dpi/72))

	var bbox C.fz_irect
	bounds = C.fz_transform_rect(bounds, ctm)
	bbox = C.fz_round_rect(bounds)

	pixmap := C.fz_new_pixmap_with_bbox(f.ctx, C.fz_device_rgb(f.ctx), bbox, nil, 1)
	if pixmap == nil {
		return nil, ErrCreatePixmap
	}

	C.fz_clear_pixmap_with_value(f.ctx, pixmap, C.int(0xff))
	defer C.fz_drop_pixmap(f.ctx, pixmap)

	device := C.fz_new_draw_device(f.ctx, ctm, pixmap)
	C.fz_enable_device_hints(f.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(f.ctx, device)

	drawMatrix := C.fz_identity
	ret := C.run_page_contents(f.ctx, page, device, drawMatrix, nil)
	if ret == 0 {
		return nil, ErrRunPageContents
	}
	ret = C.run_page_annots(f.ctx, page, device, drawMatrix, nil)
	if ret == 0 {
		return nil, ErrRunPageContents
	}
	ret = C.run_page_widgets(f.ctx, page, device, drawMatrix, nil)
	if ret == 0 {
		return nil, ErrRunPageContents
	}

	C.fz_close_device(f.ctx, device)

	buf := C.fz_new_buffer_from_pixmap_as_png(f.ctx, pixmap, C.fz_default_color_params)
	defer C.fz_drop_buffer(f.ctx, buf)

	size := C.fz_buffer_storage(f.ctx, buf, nil)
	str := C.GoStringN(C.fz_string_from_buffer(f.ctx, buf), C.int(size))

	return []byte(str), nil
}

// Links returns slice of links for given page number.
func (f *Document) Links(pageNumber int) ([]Link, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.numPageLocked() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	links := C.fz_load_links(f.ctx, page)
	defer C.fz_drop_link(f.ctx, links)

	linkCount := 0
	for currLink := links; currLink != nil; currLink = currLink.next {
		linkCount++
	}

	if linkCount == 0 {
		return nil, nil
	}

	gLinks := make([]Link, linkCount)

	currLink := links
	for i := 0; i < linkCount; i++ {
		gLinks[i] = Link{
			URI: C.GoString(currLink.uri),
		}
		currLink = currLink.next
	}

	return gLinks, nil
}

// Text returns text for given page number.
func (f *Document) Text(pageNumber int) (string, error) {
	worker, err := f.snapshotPageWorker(pageNumber, false)
	if err != nil {
		return "", err
	}
	defer worker.close()

	var opts C.fz_stext_options
	opts.flags = C.FZ_STEXT_PRESERVE_IMAGES

	text := C.new_stext_page_from_display_list_safe(worker.ctx, worker.list, &opts)
	if text == nil {
		return "", ErrRunDisplayList
	}
	defer C.fz_drop_stext_page(worker.ctx, text)

	buf := C.fz_new_buffer(worker.ctx, 1024)
	defer C.fz_drop_buffer(worker.ctx, buf)

	out := C.fz_new_output_with_buffer(worker.ctx, buf)
	defer C.fz_drop_output(worker.ctx, out)

	C.fz_print_stext_page_as_html(worker.ctx, out, text, C.int(pageNumber))
	C.fz_close_output(worker.ctx, out)

	htmlContent := C.GoString(C.fz_string_from_buffer(worker.ctx, buf))
	return htmlToText(htmlContent), nil
}

// HTML returns html for given page number.
func (f *Document) HTML(pageNumber int, header bool) (string, error) {
	worker, err := f.snapshotPageWorker(pageNumber, false)
	if err != nil {
		return "", err
	}
	defer worker.close()

	var opts C.fz_stext_options
	opts.flags = C.FZ_STEXT_PRESERVE_IMAGES

	text := C.new_stext_page_from_display_list_safe(worker.ctx, worker.list, &opts)
	if text == nil {
		return "", ErrRunDisplayList
	}
	defer C.fz_drop_stext_page(worker.ctx, text)

	buf := C.fz_new_buffer(worker.ctx, 1024)
	defer C.fz_drop_buffer(worker.ctx, buf)

	out := C.fz_new_output_with_buffer(worker.ctx, buf)
	defer C.fz_drop_output(worker.ctx, out)

	if header {
		C.fz_print_stext_header_as_html(worker.ctx, out)
	}
	C.fz_print_stext_page_as_html(worker.ctx, out, text, C.int(pageNumber))
	if header {
		C.fz_print_stext_trailer_as_html(worker.ctx, out)
	}

	C.fz_close_output(worker.ctx, out)

	str := C.GoString(C.fz_string_from_buffer(worker.ctx, buf))

	return str, nil
}

// SVGOptions controls SVG output behavior.
type SVGOptions struct {
	// TextAsText emits text as <text> elements instead of paths.
	TextAsText bool
	// ReuseImages shares image resources using <symbol> definitions.
	ReuseImages bool
	// Resolution is used when rasterizing shadings to images. Zero uses the MuPDF default.
	Resolution int
}

// SVG returns svg document for given page number.
func (f *Document) SVG(pageNumber int) (string, error) {
	return f.SVGWithAnnots(pageNumber, nil)
}

// SVGWithAnnots returns svg document for given page number including annotations/widgets.
// Options may be nil to use defaults.
func (f *Document) SVGWithAnnots(pageNumber int, opts *SVGOptions) (string, error) {
	worker, err := f.snapshotPageWorker(pageNumber, true)
	if err != nil {
		return "", err
	}
	defer worker.close()

	var bounds C.fz_rect
	bounds = C.fz_bound_display_list(worker.ctx, worker.list)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(72.0/72), C.float(72.0/72))
	bounds = C.fz_transform_rect(bounds, ctm)

	buf := C.fz_new_buffer(worker.ctx, 1024)
	defer C.fz_drop_buffer(worker.ctx, buf)

	out := C.fz_new_output_with_buffer(worker.ctx, buf)
	defer C.fz_drop_output(worker.ctx, out)

	textFormat := C.int(C.FZ_SVG_TEXT_AS_TEXT)
	reuseImages := C.int(1)
	resolution := C.int(0)
	if opts != nil {
		if opts.TextAsText {
			textFormat = C.int(C.FZ_SVG_TEXT_AS_TEXT)
		}
		if !opts.ReuseImages {
			reuseImages = C.int(0)
		}
		if opts.Resolution > 0 {
			resolution = C.int(opts.Resolution)
		}
	}
	device := C.new_svg_device_opts(worker.ctx, out, bounds.x1-bounds.x0, bounds.y1-bounds.y0, textFormat, reuseImages, resolution)
	C.fz_enable_device_hints(worker.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(worker.ctx, device)

	var cookie C.fz_cookie
	ret := C.run_display_list_safe(worker.ctx, worker.list, device, ctm, &cookie)
	if ret == 0 {
		return "", ErrRunDisplayList
	}

	C.fz_close_device(worker.ctx, device)
	C.fz_close_output(worker.ctx, out)

	str := C.GoString(C.fz_string_from_buffer(worker.ctx, buf))

	return str, nil
}

// ToC returns the table of contents (also known as outline).
func (f *Document) ToC() ([]Outline, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	data := make([]Outline, 0)

	outline := C.fz_load_outline(f.ctx, f.doc)
	if outline == nil {
		return nil, ErrLoadOutline
	}
	defer C.fz_drop_outline(f.ctx, outline)

	var walk func(outline *C.fz_outline, level int)

	walk = func(outline *C.fz_outline, level int) {
		for outline != nil {
			res := Outline{}
			res.Level = level
			res.Title = C.GoString(outline.title)
			res.URI = C.GoString(outline.uri)
			res.Page = int(outline.page.page)
			res.Top = float64(outline.y)
			data = append(data, res)

			if outline.down != nil {
				walk(outline.down, level+1)
			}
			outline = outline.next
		}
	}

	walk(outline, 1)
	return data, nil
}

// Metadata returns the map with standard metadata.
func (f *Document) Metadata() map[string]string {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	data := make(map[string]string)

	lookup := func(key string) string {
		ckey := C.CString(key)
		defer C.free(unsafe.Pointer(ckey))

		buf := make([]byte, 256)
		C.fz_lookup_metadata(f.ctx, f.doc, ckey, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))

		return string(buf)
	}

	data["format"] = lookup("format")
	data["encryption"] = lookup("encryption")
	data["title"] = lookup("info:Title")
	data["author"] = lookup("info:Author")
	data["subject"] = lookup("info:Subject")
	data["keywords"] = lookup("info:Keywords")
	data["creator"] = lookup("info:Creator")
	data["producer"] = lookup("info:Producer")
	data["creationDate"] = lookup("info:CreationDate")
	data["modDate"] = lookup("info:modDate")

	return data
}

// Bound gives the Bounds of a given Page in the document.
func (f *Document) Bound(pageNumber int) (image.Rectangle, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.numPageLocked() {
		return image.Rectangle{}, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return image.Rectangle{}, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	return image.Rect(int(bounds.x0), int(bounds.y0), int(bounds.x1), int(bounds.y1)), nil
}

// Close closes the underlying fitz document.
func (f *Document) Close() error {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if f.stream != nil {
		C.fz_drop_stream(f.ctx, f.stream)
	}

	C.fz_drop_document(f.ctx, f.doc)
	C.fz_drop_context(f.ctx)

	f.data = nil

	return nil
}
