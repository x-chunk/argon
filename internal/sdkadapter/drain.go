package sdkadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
)

// DrainTimeout bounds how long a stdio server waits, after stdin closes, for
// responses to requests it has already received.
const DrainTimeout = 30 * time.Second

// drain tracks JSON-RPC requests seen on the input stream and responses
// written to the output stream. When input hits EOF, the reader holds the EOF
// back until every pending request has been answered (or the timeout or ctx
// ends), so a client that writes its requests and closes stdin still gets
// its responses. Without this, the SDK cancels in-flight calls on EOF.
//
// It works at the wire level (newline-delimited JSON) and needs no knowledge
// of SDK internals.
type drain struct {
	ctx     context.Context
	timeout time.Duration

	mu      sync.Mutex
	pending map[string]struct{}
	idle    chan struct{} // closed and replaced as pending becomes empty
}

func newDrain(ctx context.Context, timeout time.Duration) *drain {
	d := &drain{ctx: ctx, timeout: timeout, pending: make(map[string]struct{}), idle: make(chan struct{})}
	close(d.idle)
	return d
}

type message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
}

// observe calls f for each complete line in *buf, keeping the remainder.
func observe(buf *[]byte, p []byte, f func(message)) {
	*buf = append(*buf, p...)
	for {
		i := bytes.IndexByte(*buf, '\n')
		if i < 0 {
			return
		}
		line := (*buf)[:i]
		var m message
		if json.Unmarshal(line, &m) == nil && len(m.ID) > 0 && string(m.ID) != "null" {
			f(m)
		}
		*buf = (*buf)[i+1:]
	}
}

func (d *drain) request(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pending) == 0 {
		d.idle = make(chan struct{})
	}
	d.pending[id] = struct{}{}
}

func (d *drain) response(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.pending[id]; !ok {
		return
	}
	delete(d.pending, id)
	if len(d.pending) == 0 {
		close(d.idle)
	}
}

func (d *drain) wait() {
	d.mu.Lock()
	idle := d.idle
	d.mu.Unlock()
	t := time.NewTimer(d.timeout)
	defer t.Stop()
	select {
	case <-idle:
	case <-t.C:
	case <-d.ctx.Done():
	}
}

type drainReader struct {
	d   *drain
	r   io.ReadCloser
	buf []byte
}

func (r *drainReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	observe(&r.buf, p[:n], func(m message) {
		if m.Method != "" { // requests carry a method; responses do not
			r.d.request(string(m.ID))
		}
	})
	if err == io.EOF {
		r.d.wait()
	}
	return n, err
}

func (r *drainReader) Close() error { return r.r.Close() }

type drainWriter struct {
	d   *drain
	w   io.WriteCloser
	mu  sync.Mutex
	buf []byte
}

func (w *drainWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.mu.Lock()
	observe(&w.buf, p[:n], func(m message) {
		if m.Method == "" {
			w.d.response(string(m.ID))
		}
	})
	w.mu.Unlock()
	return n, err
}

func (w *drainWriter) Close() error { return w.w.Close() }
