package puddle

import (
	"container/list"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	idleTimeout = time.Second * 2
)

// List wraps `container/list`
type List struct {
	list.List
}

// PopFront removes then returns first element in list
func (l *List) PopFront() func() {
	f := l.Front()
	l.Remove(f)
	return f.Value.(func())
}

// New creates a new puddle (aka worker pool)
func New(maxWorkers int) Puddle {
	// There must be at least one worker
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	p := &puddle{
		maxWorkers: maxWorkers,
		jobs:       make(chan func(), 1),
		workers:    make(chan func()),
		killswitch: make(chan struct{}),
	}

	// Start accepting/working jobs as they come in
	go p.serve()

	return p
}

// Puddle knows how to interact with worker pools
type Puddle interface {
	Job(f func())
	Seal()
}

// puddle is a worker pool that holds workers, tasks, and misc metadata
type puddle struct {
	maxWorkers int
	jobs       chan func()
	workers    chan func()
	killswitch chan struct{}
	queue      List
	once       sync.Once
	stopped    int32
	waiting    int32
	wait       bool
}

// Job submits a new task to the worker pool
func (p *puddle) Job(f func()) {
	if f != nil {
		p.jobs <- f
	}
}

// Seal stops worker pool and waits for queued tasks to complete
func (p *puddle) Seal() {
	p.stop(true)
}

func (p *puddle) stop(wait bool) {
	p.once.Do(func() {
		p.wait = wait
		// Close task queue and wait for currently running tasks to finish
		close(p.jobs)
	})
	<-p.killswitch
}

func (p *puddle) killWorkerIfIdle() bool {
	select {
	case p.workers <- nil:
		// Kill worker
		return true
	default:
		// No ready workers
		return false
	}
}

// process puts new jobs onto the queue, and removes jobs from the queue as workers become available.
// Returns false if puddle is stopped.
func (p *puddle) process() bool {
	select {
	case task, ok := <-p.jobs:
		if !ok {
			return false
		}
		p.queue.PushBack(task)
	case p.workers <- p.queue.Front().Value.(func()):
		// Give task to ready worker
		p.queue.PopFront()
	}
	return true
}

func (p *puddle) serve() {
	defer close(p.killswitch)
	timeout := time.NewTimer(idleTimeout)
	var workerCount int
	var idle bool

Serving:
	for {
		if p.queue.Len() != 0 {
			if !p.process() {
				break Serving
			}
			continue
		}

		select {
		case job, ok := <-p.jobs:
			if !ok {
				break Serving
			}

			// Give a task to our workers
			select {
			case p.workers <- job:
			default:
				// If we are not maxed on workers, create a new one
				if workerCount < p.maxWorkers {
					go startJob(job, p.workers)
					workerCount++
				} else {
					// Place a task on the back of the queue
					p.queue.PushBack(job)
				}
			}
			idle = false
		case <-timeout.C:
			// Timed out waiting for work to arrive.  Kill a ready worker if
			// pool has been idle for a whole timeout.
			if idle && workerCount > 0 {
				if p.killWorkerIfIdle() {
					workerCount--
				}
			}
			idle = true
			timeout.Reset(idleTimeout)
		}
	}

	// Allow queued jobs to complete
	if p.wait {
		p.work()
	}

	// Stop all workers before shutting down
	for workerCount > 0 {
		p.workers <- nil
		workerCount--
	}

	timeout.Stop()
}

// work removes each task from the waiting queue and gives it to
// workers until queue is empty.
func (p *puddle) work() {
	for p.queue.Len() != 0 {
		// A worker is ready, so give task to worker.
		p.workers <- p.queue.PopFront()
	}
}

// startJob runs initial task, then starts a worker waiting for more.
func startJob(job func(), workerQueue chan func()) {
	job()
	go worker(workerQueue)
}

// worker executes tasks and stops when it receives a nil task.
func worker(queue chan func()) {
	for job := range queue {
		if job == nil {
			return
		}
		job()
	}
}

func main() {
	pool := New(3)

	pool.Job(func() {
		c := &http.Client{}
		r, e := c.Get("http://google.com")
		if e != nil {
			panic(e.Error())
		}
		fmt.Printf("To google.com %d\n", r.StatusCode)
	})

	pool.Job(func() {
		c := &http.Client{}
		r, e := c.Get("http://yahoo.com")
		if e != nil {
			panic(e.Error())
		}
		fmt.Printf("To yahoo.com %d\n", r.StatusCode)
	})

	pool.Job(func() {
		c := &http.Client{}
		r, e := c.Get("http://example.com")
		if e != nil {
			panic(e.Error())
		}
		fmt.Printf("To example.com %d\n", r.StatusCode)
	})

	pool.Seal()
}
