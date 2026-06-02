package tts

import (
	"context"
	"sync"
)

type JobStatus string

const (
	JobStatusQueued  JobStatus = "queued"
	JobStatusRunning JobStatus = "running"
)

type JobQueue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	queued   []int64
	statuses map[int64]JobStatus
}

func NewJobQueue() *JobQueue {
	q := &JobQueue{
		statuses: make(map[int64]JobStatus),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *JobQueue) Enqueue(entryID int64) JobStatus {
	q.mu.Lock()
	defer q.mu.Unlock()

	if status, ok := q.statuses[entryID]; ok {
		return status
	}

	q.queued = append(q.queued, entryID)
	q.statuses[entryID] = JobStatusQueued
	q.cond.Signal()
	return JobStatusQueued
}

func (q *JobQueue) Next(ctx context.Context) (int64, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.queued) == 0 {
		if ctx.Err() != nil {
			return 0, false
		}
		q.cond.Wait()
	}

	entryID := q.queued[0]
	copy(q.queued, q.queued[1:])
	q.queued = q.queued[:len(q.queued)-1]
	q.statuses[entryID] = JobStatusRunning
	return entryID, true
}

func (q *JobQueue) Complete(entryID int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.statuses, entryID)
}

func (q *JobQueue) Status(entryID int64) (JobStatus, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	status, ok := q.statuses[entryID]
	return status, ok
}

func (q *JobQueue) Wake() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.cond.Broadcast()
}
