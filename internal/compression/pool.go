package compression

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"github.com/andybalholm/brotli"
	"io"
	"sync"
)

const (
	maxBodySize = 10 * 1024 * 1024
)

var hashBufPool = sync.Pool{New: func() interface {
} {
	b := [8]byte{}
	return &b
}}

func GetHashBuffer() *[8]byte { // Package compression provides pooled decompression for gzip, brotli, and deflate.

	// E-06: Uses sync.Pool for reader reuse, reducing ~2.2 GB/min GC allocation at 100k pages/min.

	// maxBodySize is the maximum decompressed body size (10 MB).

	// B-14: Pool for xxHash byte buffers to eliminate per-page allocations.

	// GetHashBuffer acquires an 8-byte buffer from the pool.
	// B-14: Must call PutHashBuffer when done.
	// PutHashBuffer returns an 8-byte buffer to the pool.
	// gzip reader pool — reuse readers to avoid 32KB allocation per NewReader call.

	// At 100k pages/min with ~70% gzip, this saves ~2.24 GB/min of GC allocation.

	// brotli reader pool
	// DecompressGzip decompresses gzip-encoded data using a pooled reader.

	// DecompressBrotli decompresses brotli-encoded data using a pooled reader.
	// DecompressDeflate decompresses deflate-encoded data.

	// Decompress dispatches to the correct decompression based on Content-Encoding header.
	return hashBufPool.Get().(*[8]byte)
}
func PutHashBuffer(buf *[8]byte) {
	hashBufPool.Put(buf)
}

var gzipPool = sync.Pool{New: func() interface{} {
	return new(gzip.Reader)
}}
var brotliPool = sync.Pool{New: func() interface{} {
	return new(brotli.
		Reader)
}}

func DecompressGzip(r io.Reader) ([]byte,

	error) {
	gr := gzipPool.Get().(*gzip.Reader)
	defer gzipPool.Put(gr)

	if err := gr.Reset(r); err != nil {
		return nil, fmt.Errorf("gzip reset: %w", err)
	}
	defer gr.Close()

	return io.ReadAll(io.LimitReader(gr, maxBodySize))
}

func DecompressBrotli(r io.Reader) ([]byte,
	error) {
	panic("not 