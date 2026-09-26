package handler

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"testing"
)

func TestSSEFirstResponseBodyBytePreventsRetry(t *testing.T) {
	source, err := os.ReadFile("openai_shared_pool_chat.go")
	if err != nil {
		t.Fatalf("post-commit retry: read shared chat handler: %v", err)
	}
	if bytes.Contains(source, []byte("shouldFailoverSharedPoolRoute(")) || bytes.Contains(source, []byte("writerSizeBeforeForward")) {
		t.Fatal("post-commit retry: SSE comment or heartbeat response-body byte is not a terminal commit boundary")
	}
}

type partialCommitWriter struct{}

func (partialCommitWriter) Write(body []byte) (int, error) {
	if len(body) == 0 {
		return 0, nil
	}
	return 1, io.ErrUnexpectedEOF
}

func TestCommitBoundaryCommitsOnFirstAcceptedBodyByte(t *testing.T) {
	var callbacks atomic.Int32
	boundary, err := NewCommitBoundary(func() error {
		callbacks.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("new commit boundary: %v", err)
	}

	if _, err := boundary.WriteBody(io.Discard, nil); err != nil {
		t.Fatalf("write empty body: %v", err)
	}
	if boundary.Committed() {
		t.Fatal("empty body committed the attempt")
	}
	if _, err := boundary.WriteBody(io.Discard, []byte(": heartbeat\n\n")); err != nil {
		t.Fatalf("write SSE heartbeat: %v", err)
	}
	if _, err := boundary.WriteBody(io.Discard, []byte("data: later\n\n")); err != nil {
		t.Fatalf("write later SSE data: %v", err)
	}
	if !boundary.Committed() || callbacks.Load() != 1 {
		t.Fatalf("commit state=%v callbacks=%d, want committed once", boundary.Committed(), callbacks.Load())
	}
}

func TestCommitBoundaryCommitsOnPartialWriteAndApplicationFrame(t *testing.T) {
	var bodyCallbacks atomic.Int32
	bodyBoundary, err := NewCommitBoundary(func() error {
		bodyCallbacks.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("new body boundary: %v", err)
	}

	n, err := bodyBoundary.WriteBody(partialCommitWriter{}, []byte("response"))
	if n != 1 || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("partial write n=%d err=%v", n, err)
	}
	if !bodyBoundary.Committed() || bodyCallbacks.Load() != 1 {
		t.Fatal("partial body byte did not commit exactly once")
	}

	var frameCallbacks atomic.Int32
	frameBoundary, err := NewCommitBoundary(func() error {
		frameCallbacks.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("new frame boundary: %v", err)
	}
	if err := frameBoundary.ObserveApplicationFrame(); err != nil {
		t.Fatalf("observe first application frame: %v", err)
	}
	if err := frameBoundary.ObserveApplicationFrame(); err != nil {
		t.Fatalf("observe later application frame: %v", err)
	}
	if !frameBoundary.Committed() || frameCallbacks.Load() != 1 {
		t.Fatal("application frame did not commit exactly once")
	}
}

func TestCommitBoundaryRemainsCommittedWhenCallbackFails(t *testing.T) {
	callbackErr := errors.New("commit record failed")
	boundary, err := NewCommitBoundary(func() error { return callbackErr })
	if err != nil {
		t.Fatalf("new commit boundary: %v", err)
	}

	n, err := boundary.WriteBody(io.Discard, []byte("x"))
	if n != 1 || !errors.Is(err, callbackErr) {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	if !boundary.Committed() {
		t.Fatal("visible response byte must remain committed after callback failure")
	}
}
