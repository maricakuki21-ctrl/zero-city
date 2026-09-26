package service

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

const geminiSignalBufferLimit = 1 << 20

// The upstream scanners can outlive the request on cancellation. Reads only
// update private state under a mutex; finalization touches Gin on its owner.
type geminiResponseObserver struct {
	io.ReadCloser
	mu          sync.Mutex
	sse         bool
	finished    bool
	eof         bool
	truncated   bool
	lineDropped bool
	eventDrop   bool
	sawData     bool
	line        []byte
	data        []byte
	body        []byte
	best        geminiResponseSignal
}

func observeGeminiResponse(c *gin.Context, resp *http.Response, account *Account, upstreamStream, clientStream bool, requestID string) func() {
	observer := &geminiResponseObserver{ReadCloser: resp.Body, sse: upstreamStream}
	resp.Body = observer
	return func() {
		recordGeminiResponseSignal(c, account, observer.finish(), clientStream, requestID)
	}
}

func (o *geminiResponseObserver) Read(p []byte) (int, error) {
	n, err := o.ReadCloser.Read(p)
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.finished {
		return n, err
	}
	if !o.sse {
		o.appendBody(p[:n])
	} else {
		for _, b := range p[:n] {
			if b == '\n' {
				o.consumeLine()
				o.line = o.line[:0]
				o.lineDropped = false
			} else if !o.lineDropped {
				if len(o.line) == geminiSignalBufferLimit {
					o.line = nil
					o.lineDropped = true
					o.eventDrop = true
					o.truncated = true
				} else {
					o.line = append(o.line, b)
				}
			}
		}
	}
	if errors.Is(err, io.EOF) {
		o.eof = true
		if o.sse {
			if len(o.line) > 0 || o.lineDropped {
				o.consumeLine()
			}
			o.consumeEvent()
		}
	}
	return n, err
}

func (o *geminiResponseObserver) appendBody(p []byte) {
	if o.truncated {
		return
	}
	if len(o.body)+len(p) > geminiSignalBufferLimit {
		o.body = nil
		o.truncated = true
		return
	}
	o.body = append(o.body, p...)
}

func (o *geminiResponseObserver) consumeLine() {
	if o.lineDropped {
		return
	}
	line := bytes.TrimSuffix(o.line, []byte("\r"))
	if len(line) == 0 {
		o.consumeEvent()
		return
	}
	if bytes.HasPrefix(line, []byte("data:")) {
		o.sawData = true
		data := bytes.TrimPrefix(line, []byte("data:"))
		data = bytes.TrimPrefix(data, []byte(" "))
		if !o.eventDrop {
			if len(o.data)+len(data)+1 > geminiSignalBufferLimit {
				o.data = nil
				o.eventDrop = true
			} else {
				o.data = append(o.data, data...)
				o.data = append(o.data, '\n')
			}
		}
		return
	}
	if bytes.HasPrefix(line, []byte(":")) || bytes.HasPrefix(line, []byte("event:")) ||
		bytes.HasPrefix(line, []byte("id:")) || bytes.HasPrefix(line, []byte("retry:")) {
		return
	}
	o.appendBody(line)
	o.appendBody([]byte("\n"))
}

func (o *geminiResponseObserver) consumeEvent() {
	if !o.eventDrop {
		if signal, ok := detectGeminiResponseSignalInBody(o.data); ok && signal.Kind > o.best.Kind {
			o.best = signal
		}
	}
	o.data = o.data[:0]
	o.eventDrop = false
}

func (o *geminiResponseObserver) finish() geminiResponseSignal {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.finished {
		return geminiResponseSignal{}
	}
	o.finished = true
	if o.best.Kind != geminiSignalNone {
		return o.best
	}
	// Partial reads, cancellation and buffer limits cannot establish emptiness.
	if !o.eof || o.truncated || (o.sse && o.sawData) {
		return geminiResponseSignal{}
	}
	if signal, ok := detectGeminiResponseSignalInBody(o.body); ok {
		return signal
	}
	if !isGeminiEmptyResponseBody(o.body) {
		return geminiResponseSignal{}
	}
	reason, noun := "EMPTY_RESPONSE", "response"
	if o.sse {
		reason, noun = "EMPTY_STREAM", "stream"
	}
	return geminiResponseSignal{Kind: geminiSignalAbnormalStop, Reason: reason,
		Message: strings.Join([]string{"Gemini upstream returned an empty", noun, "body"}, " "),
		Status:  http.StatusBadGateway}
}
