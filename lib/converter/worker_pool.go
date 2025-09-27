package converter

import (
	"context"
	"fmt"
	"sync"
)

// TaskWithContext represents a task that accepts context and returns data and an error.
type TaskWithContext func(ctx context.Context) ([]byte, error)

// WorkerPool manages a pool of goroutines for concurrent task execution.
type WorkerPool struct {
	maxWorkers   int
	contextQueue chan contextTask
	workerWg     sync.WaitGroup
	quit         chan struct{}
	started      bool
	mu           sync.RWMutex
}

type contextTask struct {
	ctx  context.Context
	task TaskWithContext
	done chan taskResult
}

type taskResult struct {
	data []byte
	err  error
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	return &WorkerPool{
		maxWorkers:   maxWorkers,
		contextQueue: make(chan contextTask, maxWorkers*2), // Buffered queue
		quit:         make(chan struct{}),
	}
}

// Start initializes and starts all workers.
func (p *WorkerPool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return fmt.Errorf("worker pool already started")
	}
	for i := 0; i < p.maxWorkers; i++ {
		p.workerWg.Add(1)
		go p.worker()
	}
	p.started = true
	return nil
}

// worker is the main goroutine that processes tasks.
func (p *WorkerPool) worker() {
	defer p.workerWg.Done()
	for {
		select {
		case ctxTask := <-p.contextQueue:
			if ctxTask.task == nil {
				continue
			}
			data, err := ctxTask.task(ctxTask.ctx)
			// Send result back
			select {
			case ctxTask.done <- taskResult{data: data, err: err}:
			case <-ctxTask.ctx.Done():
			}
		case <-p.quit:
			return
		}
	}
}

// SubmitWithContext submits a task with context and returns a channel for the result.
func (p *WorkerPool) SubmitWithContext(ctx context.Context, task TaskWithContext) (<-chan taskResult, error) {
	p.mu.RLock()
	if !p.started {
		p.mu.RUnlock()
		return nil, fmt.Errorf("worker pool not started")
	}
	p.mu.RUnlock()

	done := make(chan taskResult, 1)
	ctxTask := contextTask{
		ctx:  ctx,
		task: task,
		done: done,
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p.contextQueue <- ctxTask:
		return done, nil
	default:
		// Fallback: Queue is full, execute in a new goroutine to avoid blocking.
		go func() {
			data, err := task(ctx)
			select {
			case done <- taskResult{data: data, err: err}:
			case <-ctx.Done():
			}
		}()
		return done, nil
	}
}

// Stop gracefully shuts down the worker pool.
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return
	}
	close(p.quit)
	p.workerWg.Wait()
	p.started = false
}
