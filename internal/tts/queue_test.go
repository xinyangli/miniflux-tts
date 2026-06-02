package tts

import (
	"context"
	"testing"
)

func TestJobQueueFIFOAndStatusTransitions(t *testing.T) {
	queue := NewJobQueue()

	if status := queue.Enqueue(1); status != JobStatusQueued {
		t.Fatalf("expected queued status, got %q", status)
	}
	if status := queue.Enqueue(2); status != JobStatusQueued {
		t.Fatalf("expected queued status, got %q", status)
	}
	if status := queue.Enqueue(1); status != JobStatusQueued {
		t.Fatalf("expected duplicate queued status, got %q", status)
	}

	entryID, ok := queue.Next(context.Background())
	if !ok {
		t.Fatal("expected first job")
	}
	if entryID != 1 {
		t.Fatalf("expected first job to be entry 1, got %d", entryID)
	}
	if status := queue.Enqueue(1); status != JobStatusRunning {
		t.Fatalf("expected duplicate running status, got %q", status)
	}

	entryID, ok = queue.Next(context.Background())
	if !ok {
		t.Fatal("expected second job")
	}
	if entryID != 2 {
		t.Fatalf("expected second job to be entry 2, got %d", entryID)
	}

	queue.Complete(1)
	if status := queue.Enqueue(1); status != JobStatusQueued {
		t.Fatalf("expected completed job to be re-enqueued, got %q", status)
	}

	entryID, ok = queue.Next(context.Background())
	if !ok {
		t.Fatal("expected re-enqueued job")
	}
	if entryID != 1 {
		t.Fatalf("expected re-enqueued entry 1, got %d", entryID)
	}
}

func TestJobQueueNextStopsOnContextCancel(t *testing.T) {
	queue := NewJobQueue()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if entryID, ok := queue.Next(ctx); ok || entryID != 0 {
		t.Fatalf("expected canceled dequeue, got entry_id=%d ok=%v", entryID, ok)
	}
}
