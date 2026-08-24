package application

import (
	"context"
	"sync"
)

type mailbox struct{ requests chan mailboxRequest }
type mailboxRequest struct {
	run    func() (any, error)
	answer chan mailboxAnswer
}
type mailboxAnswer struct {
	value any
	err   error
}
type mailboxRegistry struct {
	mu    sync.Mutex
	boxes map[string]*mailbox
}

func newMailboxRegistry() *mailboxRegistry { return &mailboxRegistry{boxes: map[string]*mailbox{}} }
func (r *mailboxRegistry) forBatch(id string) *mailbox {
	r.mu.Lock()
	defer r.mu.Unlock()
	box := r.boxes[id]
	if box == nil {
		box = &mailbox{requests: make(chan mailboxRequest)}
		r.boxes[id] = box
		go box.loop()
	}
	return box
}
func (m *mailbox) loop() {
	for request := range m.requests {
		value, err := request.run()
		request.answer <- mailboxAnswer{value, err}
	}
}
func (m *mailbox) do(ctx context.Context, run func() (any, error)) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	answer := make(chan mailboxAnswer, 1)
	m.requests <- mailboxRequest{run: run, answer: answer}
	select {
	case result := <-answer:
		return result.value, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
