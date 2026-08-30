package run

import (
	"context"
	"io"
	"sync"
)

type eventQueue struct {
	mu          sync.Mutex
	structural  []Event
	output      map[string]Event
	outputOrder []string
	closed      bool
	ready       chan struct{}
}

func newEventQueue() *eventQueue {
	return &eventQueue{
		output: make(map[string]Event),
		ready:  make(chan struct{}, 1),
	}
}

func (q *eventQueue) push(event Event) {
	q.mu.Lock()
	if !q.closed {
		q.structural = append(q.structural, event)
		q.signalLocked()
	}
	q.mu.Unlock()
}

func (q *eventQueue) pushOutput(targetID string, event Event) {
	q.mu.Lock()
	if !q.closed {
		if _, exists := q.output[targetID]; !exists {
			q.outputOrder = append(q.outputOrder, targetID)
		}
		q.output[targetID] = event
		q.signalLocked()
	}
	q.mu.Unlock()
}

func (q *eventQueue) pushFinished(targetID string, event Event) {
	q.mu.Lock()
	if !q.closed {
		if pending, exists := q.output[targetID]; exists {
			q.structural = append(q.structural, pending)
			delete(q.output, targetID)
			q.removeOutputOrderLocked(targetID)
		}
		q.structural = append(q.structural, event)
		q.signalLocked()
	}
	q.mu.Unlock()
}

func (q *eventQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.signalLocked()
	q.mu.Unlock()
}

func (q *eventQueue) next(ctx context.Context) (Event, error) {
	for {
		q.mu.Lock()
		if event, ok := q.popLocked(); ok {
			if q.hasEventsLocked() || q.closed {
				q.signalLocked()
			}
			q.mu.Unlock()
			return event, nil
		}
		if q.closed {
			q.mu.Unlock()
			return Event{}, io.EOF
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return Event{}, ctx.Err()
		case <-q.ready:
		}
	}
}

func (q *eventQueue) popLocked() (Event, bool) {
	if len(q.structural) > 0 {
		event := q.structural[0]
		q.structural = q.structural[1:]
		return event, true
	}
	for len(q.outputOrder) > 0 {
		targetID := q.outputOrder[0]
		q.outputOrder = q.outputOrder[1:]
		event, ok := q.output[targetID]
		delete(q.output, targetID)
		if ok {
			return event, true
		}
	}
	return Event{}, false
}

func (q *eventQueue) removeOutputOrderLocked(targetID string) {
	for i, id := range q.outputOrder {
		if id == targetID {
			q.outputOrder = append(q.outputOrder[:i], q.outputOrder[i+1:]...)
			return
		}
	}
}

func (q *eventQueue) hasEventsLocked() bool {
	return len(q.structural) > 0 || len(q.output) > 0
}

func (q *eventQueue) signalLocked() {
	select {
	case q.ready <- struct{}{}:
	default:
	}
}
