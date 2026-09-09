package downloader

import "testing"

func TestQueueController(t *testing.T) {
	qc := NewQueueController(1)
	if qc.GetMaxActiveGames() != 1 {
		t.Fatalf("expected max active 1, got %d", qc.GetMaxActiveGames())
	}

	if !qc.CanStartNext(0) {
		t.Fatal("expected CanStartNext(0) to be true for max 1")
	}

	if qc.CanStartNext(1) {
		t.Fatal("expected CanStartNext(1) to be false for max 1")
	}

	qc.SetMaxActiveGames(3)
	if !qc.CanStartNext(1) {
		t.Fatal("expected CanStartNext(1) to be true for max 3")
	}
	if !qc.CanStartNext(2) {
		t.Fatal("expected CanStartNext(2) to be true for max 3")
	}
	if qc.CanStartNext(3) {
		t.Fatal("expected CanStartNext(3) to be false for max 3")
	}
}

func TestBufferPool(t *testing.T) {
	buf1 := GetChunkBuffer()
	if buf1 == nil || len(*buf1) != ChunkBufferSize {
		t.Fatalf("expected buffer of size %d, got %v", ChunkBufferSize, buf1)
	}

	PutChunkBuffer(buf1)

	buf2 := GetChunkBuffer()
	if buf2 == nil || len(*buf2) != ChunkBufferSize {
		t.Fatalf("expected buffer from pool, got %v", buf2)
	}
	PutChunkBuffer(buf2)
}
