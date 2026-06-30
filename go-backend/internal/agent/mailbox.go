package agent

import (
	"context"
	"sync"
)

type WorkerResult struct {
	Task   Task
	Result AgentResult
	Err    error
}

type Mailbox struct {
	results chan WorkerResult
	once    sync.Once
}

func NewMailbox(buffer int) *Mailbox {
	if buffer < 0 {
		buffer = 0
	}
	return &Mailbox{
		results: make(chan WorkerResult, buffer),
	}
}

func (m *Mailbox) Send(ctx context.Context, result WorkerResult) bool {
	if m == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return false
	case m.results <- result:
		return true
	}
}

func (m *Mailbox) Results() <-chan WorkerResult {
	if m == nil {
		ch := make(chan WorkerResult)
		close(ch)
		return ch
	}
	return m.results
}

func (m *Mailbox) Close() {
	if m == nil {
		return
	}
	m.once.Do(func() {
		close(m.results)
	})
}

func (m *Mailbox) Collect() []WorkerResult {
	results := []WorkerResult{}
	for result := range m.Results() {
		results = append(results, result)
	}
	return results
}
