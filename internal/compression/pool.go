// Package compression provides pooled decompression for gzip, brotli, and deflate.
// E-06: Uses sync.Pool for reader reuse, reducing ~2.2 GB/min GC allocation at 100k pages/min.
package compression

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
)

const (
	// maxBodySize is the maximum decompressed body size (10 MB).
	maxBodySize = 10 * 1024 * 1024
)

// B-14: Pool for xxHash byte buffers to eliminate per-page allocations.
var hashBufPool = sync.Pool{
	New: func() interface{} {
		b := [8]byte{}
		return &b
	},
}

// GetHashBuffer acquires an 8-byte buffer from the pool.
// B-14: Must call PutHashBuffer when done.
func GetHashBuffer() *[8]byte {
	return hashBufPool.Get().(*[8]byte)
}

// PutHashBuffer returns an 8-byte buffer to the pool.
func PutHashBuffer(buf *[8]byte) {
	hashBufPool.Put(buf)
}

// gzip reader pool — reuse readers to avoid 32KB allocation per NewReader call.
// At 100k pages/min with ~70% gzip, this saves ~2.24 GB/min of GC allocation.
var gzipPool = sync.Pool{
	New: func() interface{} {
		return new(gzip.Reader)
	},
}

// brotli reader pool
var brotliPool = sync.Pool{
	New: func() interface{} {
		return new(brotli.Reader)
	},
}

// DecompressGzip decompresses gzip-encoded data using a pooled reader.
func DecompressGzip(r io.Reader) ([]byte, error) {
	gr := gzipPool.Get().(*gzip.Reader)
	defer gzipPool.Put(gr)

	if err := gr.Reset(r); err != nil {
		return nil, fmt.Errorf("gzip reset: %w", err)
	}
	defer gr.Close()

	return io.ReadAll(io.LimitReader(gr, maxBodySize))
}

// DecompressBrotli decompresses brotli-encoded data using a pooled reader.
func DecompressBrotli(r io.Reader) ([]byte, error) {
	br := brotliPool.Get().(*brotli.Reader)
	defer brotliPool.Put(br)

	br.Reset(r)
	return io.ReadAll(io.LimitReader(br, maxBodySize))
}

// DecompressDeflate decompresses deflate-encoded data.
func DecompressDeflate(r io.Reader) ([]byte, error) {
	fr := flate.NewReader(r)
	defer fr.Close()

	return io.ReadAll(io.LimitReader(fr, maxBodySize))
}

// Decompress dispatches to the correct decompression based on Content-Encoding header.
func Decompress(encoding string, r io.Reader) ([]byte, error) {
	switch encoding {
	case "gzip":
		return DecompressGzip(r)
	case "br":
		return DecompressBrotli(r)
	case "deflate":
		return DecompressDeflate(r)
	default:
		return io.ReadAll(io.LimitReader(r, maxBodySize))
	}
}
