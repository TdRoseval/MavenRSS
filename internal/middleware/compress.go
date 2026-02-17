// Package middleware provides HTTP middleware components for the application.
package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

const (
	gzipEncoding   = "gzip"
	brotliEncoding = "br"
	acceptEncoding = "Accept-Encoding"
	contentEncoding = "Content-Encoding"
	contentType = "Content-Type"
	contentLength = "Content-Length"
	vary = "Vary"
	minCompressSize = 1024 // Only compress files larger than 1KB
)

var (
	gzipPool = sync.Pool{
		New: func() interface{} {
			return gzip.NewWriter(io.Discard)
		},
	}

	brotliPool = sync.Pool{
		New: func() interface{} {
			return brotli.NewWriter(io.Discard)
		},
	}
)

// compressibleContentTypes lists content types that should be compressed
var compressibleContentTypes = map[string]bool{
	"text/html":                true,
	"text/css":                 true,
	"text/plain":               true,
	"text/javascript":          true,
	"application/javascript":   true,
	"application/json":         true,
	"application/xml":          true,
	"application/x-javascript": true,
	"image/svg+xml":            true,
}

// Compress returns a middleware that compresses responses using gzip or brotli
func Compress() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip compression for certain paths or methods
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			// Check if client accepts compression
			acceptEnc := r.Header.Get(acceptEncoding)
			useBrotli := strings.Contains(acceptEnc, brotliEncoding)
			useGzip := strings.Contains(acceptEnc, gzipEncoding)

			if !useBrotli && !useGzip {
				next.ServeHTTP(w, r)
				return
			}

			// Create a response recorder to capture the response
			recorder := &compressResponseRecorder{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				statusCode:     http.StatusOK,
			}

			// Let the next handler write to the recorder
			next.ServeHTTP(recorder, r)

			// Check if we should compress
			if !shouldCompress(recorder) {
				// Write the original response
				for k, v := range recorder.header {
					w.Header()[k] = v
				}
				w.WriteHeader(recorder.statusCode)
				io.Copy(w, recorder.body)
				return
			}

			// Choose the best compression method
			var encoding string
			if useBrotli {
				encoding = brotliEncoding
			} else if useGzip {
				encoding = gzipEncoding
			}

			// Compress the response
			var compressed bytes.Buffer
			var err error

			if encoding == brotliEncoding {
				err = compressWithBrotli(recorder.body.Bytes(), &compressed)
			} else {
				err = compressWithGzip(recorder.body.Bytes(), &compressed)
			}

			if err != nil {
				// Fallback to uncompressed
				for k, v := range recorder.header {
					w.Header()[k] = v
				}
				w.WriteHeader(recorder.statusCode)
				io.Copy(w, recorder.body)
				return
			}

			// Copy headers except Content-Length
			for k, v := range recorder.header {
				if k != contentLength {
					w.Header()[k] = v
				}
			}

			// Set compression headers
			w.Header().Set(contentEncoding, encoding)
			w.Header().Add(vary, acceptEncoding)
			w.Header().Set(contentLength, strconv.Itoa(compressed.Len()))
			w.WriteHeader(recorder.statusCode)
			io.Copy(w, &compressed)
		})
	}
}

// compressResponseRecorder captures the response
type compressResponseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	header     http.Header
	statusCode int
	wroteHeader bool
}

func (cr *compressResponseRecorder) Header() http.Header {
	if cr.header == nil {
		cr.header = make(http.Header)
	}
	return cr.header
}

func (cr *compressResponseRecorder) WriteHeader(code int) {
	if cr.wroteHeader {
		return
	}
	cr.statusCode = code
	cr.wroteHeader = true
}

func (cr *compressResponseRecorder) Write(b []byte) (int, error) {
	if !cr.wroteHeader {
		cr.WriteHeader(http.StatusOK)
	}
	return cr.body.Write(b)
}

// shouldCompress determines if a response should be compressed
func shouldCompress(recorder *compressResponseRecorder) bool {
	// Don't compress if already compressed
	if recorder.header.Get(contentEncoding) != "" {
		return false
	}

	// Check content type
	contentType := recorder.header.Get(contentType)
	if contentType == "" {
		return false
	}

	// Extract the media type (without charset)
	mediaType := strings.Split(contentType, ";")[0]
	if !compressibleContentTypes[mediaType] {
		return false
	}

	// Check size
	if recorder.body.Len() < minCompressSize {
		return false
	}

	return true
}

// compressWithGzip compresses data using gzip
func compressWithGzip(data []byte, w io.Writer) error {
	gz := gzipPool.Get().(*gzip.Writer)
	defer gzipPool.Put(gz)
	gz.Reset(w)
	if _, err := gz.Write(data); err != nil {
		return err
	}
	return gz.Close()
}

// compressWithBrotli compresses data using brotli
func compressWithBrotli(data []byte, w io.Writer) error {
	br := brotliPool.Get().(*brotli.Writer)
	defer brotliPool.Put(br)
	br.Reset(w)
	if _, err := br.Write(data); err != nil {
		return err
	}
	return br.Close()
}
