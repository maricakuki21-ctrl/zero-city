package handler

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var ErrMissingCommitCallback = errors.New("commit boundary: missing callback")

type CommitBoundary struct {
	onCommit func() error
	once     sync.Once
	err      error
	marked   atomic.Bool
}

func NewCommitBoundary(onCommit func() error) (*CommitBoundary, error) {
	if onCommit == nil {
		return nil, ErrMissingCommitCallback
	}
	return &CommitBoundary{onCommit: onCommit}, nil
}

func (b *CommitBoundary) WriteBody(dst io.Writer, body []byte) (int, error) {
	n, writeErr := dst.Write(body)
	if n > 0 {
		commitErr := b.markCommitted()
		return n, errors.Join(writeErr, commitErr)
	}
	return n, writeErr
}

func (b *CommitBoundary) ObserveApplicationFrame() error {
	return b.markCommitted()
}

func (b *CommitBoundary) Committed() bool {
	return b.marked.Load()
}

func (b *CommitBoundary) markCommitted() error {
	b.once.Do(func() {
		b.marked.Store(true)
		b.err = b.onCommit()
	})
	return b.err
}
