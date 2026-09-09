package downloader

import "sync"

const ChunkBufferSize = 256 * 1024 // 256KB buffer for high-throughput FTP/SFTP streaming

var chunkBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, ChunkBufferSize)
		return &buf
	},
}

// GetChunkBuffer retrieves a 256KB buffer from the pool
func GetChunkBuffer() *[]byte {
	return chunkBufferPool.Get().(*[]byte)
}

// PutChunkBuffer returns a buffer back to the pool
func PutChunkBuffer(buf *[]byte) {
	if buf != nil && len(*buf) == ChunkBufferSize {
		chunkBufferPool.Put(buf)
	}
}
