package yeet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/vmihailenco/msgpack/v5"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Sock struct {
	inner     net.Conn
	interval  time.Duration
	err       error
	lossPrope atomic.Uint32

	handshakeTimeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	rbuf  bytes.Buffer
	rlock sync.Mutex

	wbuf  bytes.Buffer
	wlock sync.Mutex

	readDeadline time.Time
}

type SockOpt func(*Sock)

func Keepalive(keepalive time.Duration) SockOpt {
	return func(self *Sock) {
		self.interval = keepalive
	}
}
func HandshakeTimeout(timeout time.Duration) SockOpt {
	return func(self *Sock) {
		self.handshakeTimeout = timeout
	}
}

func Connect(inner net.Conn, opts ...SockOpt) (*Sock, error) {

	self := &Sock{
		inner:            inner,
		interval:         time.Second,
		handshakeTimeout: time.Second * 2,
	}
	self.lossPrope.Store(0)

	for _, opt := range opts {
		opt(self)
	}

	// handshake

	// this would normally not be async,
	// but we're using pipe for testing, and that doesn't buffer D:
	writeHS := make(chan error)
	go func() {
		_, err := self.writeAll([]byte{'H', 0, 0, 0})
		writeHS <- err
	}()

	var header = make([]byte, 4)
	self.inner.SetReadDeadline(time.Now().Add(self.handshakeTimeout))
	if _, err := io.ReadFull(self.inner, header); err != nil {
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	if header[0] != 'H' {
		return nil, fmt.Errorf("invalid handshake response %q", header)
	}

	var l = binary.LittleEndian.Uint16(header[2:4])
	self.rbuf.Reset()
	self.rbuf.Grow(int(l))
	self.inner.SetReadDeadline(time.Now().Add(self.handshakeTimeout))
	if _, err := io.ReadFull(self.inner, self.rbuf.Bytes()[:l]); err != nil {
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	err := <-writeHS
	if err != nil {
		return nil, fmt.Errorf("write handshake failed: %w", err)
	}

	// handshake complete, lets go

	self.ctx, self.cancel = context.WithCancel(context.Background())

	go self.pinger()

	return self, nil
}

func (self *Sock) pinger() {
	ping := time.NewTicker(self.interval)
	for {
		select {
		case <-self.ctx.Done():
			ping.Stop()
			return
		case <-ping.C:

			if self.lossPrope.Add(1) > 2 {
				err := fmt.Errorf("ping timeout")
				self.CloseWithError(err)
				return
			}

			if self.wlock.TryLock() {
				_, err := self.writeAll([]byte{'P', 0, 0, 0})
				self.wlock.Unlock()
				if err != nil {
					self.CloseWithError(err)
					return
				}
			}
		}
	}
}

func (self *Sock) CloseWithError(err error) {
	self.err = err

	if self.wlock.TryLock() {

		errstr := err.Error()
		var header = make([]byte, 4)
		header[0] = 'E'
		binary.LittleEndian.PutUint16(header[2:], uint16(len(errstr)))
		self.writeAll(append(header, errstr...))

		self.writeAll([]byte{'C', 0, 0, 0})
		self.wlock.Unlock()
	}

	self.cancel()
	self.inner.Close()
}

func (self *Sock) Close() {
	if self.wlock.TryLock() {
		self.writeAll([]byte{'C', 0, 0, 0})
		self.wlock.Unlock()
	}

	self.cancel()
	self.inner.Close()
}

// discard all incomming messages, just respond to ping
func (self *Sock) Discard() error {
	for {
		if err := self.Read(nil); err != nil {
			return err
		}
	}
}

func (self *Sock) SetReadDeadline(t time.Time) error {
	self.readDeadline = t
	return nil
}

func (self *Sock) SetDeadline(t time.Time) error {
	self.readDeadline = t
	return nil
}

func (self *Sock) Read(msg interface{}) error {

	self.rlock.Lock()
	defer self.rlock.Unlock()

	for {

		if !self.readDeadline.IsZero() {
			if time.Now().After(self.readDeadline) {
				return os.ErrDeadlineExceeded
			}
		}

		if self.err != nil {
			return self.err
		}

		var header [4]byte
		self.inner.SetReadDeadline(time.Now().Add(self.interval * 2))

		if _, err := io.ReadFull(self.inner, header[:]); err != nil {
			if self.err != nil {
				return self.err
			}
			return fmt.Errorf("failed to read header: %w", err)
		}

		self.lossPrope.Store(0)

		var l = binary.LittleEndian.Uint16(header[2:4])

		self.rbuf.Reset()
		self.rbuf.Grow(int(l))
		self.inner.SetReadDeadline(time.Now().Add(self.interval * 2))
		if _, err := io.ReadFull(self.inner, self.rbuf.Bytes()[:l]); err != nil {

			if self.err != nil {
				return self.err
			}
			return fmt.Errorf("failed to read body: %w", err)
		}

		switch kind := header[0]; {
		case kind > 'a' && kind < 'z':

			continue

		case kind == 'M':

			if self.err != nil {
				return self.err
			}

			if msg == nil {
				return nil
			}
			err := msgpack.Unmarshal(self.rbuf.Bytes()[:l], msg)
			if err != nil {
				return fmt.Errorf("failed to decode msgpack (size %d): %w", l, err)
			} else {
				return nil
			}

		case kind == 'E':

			return fmt.Errorf("remote error: %s", string(self.rbuf.Bytes()[:l]))

		case kind == 'C':

			return fmt.Errorf("remote closed connection (%w)", io.EOF)

		case kind == 'P':
			if self.wlock.TryLock() {
				_, err := self.writeAll([]byte{'P', 0, 0, 0})
				self.wlock.Unlock()
				if err != nil {
					return fmt.Errorf("failed to respond to ping: %w", err)
				}
			}
			continue

		case kind == 'R':
			continue

		default:
			return fmt.Errorf("unknown required message type: %c", kind)
		}
	}
}

func (self *Sock) Write(msg interface{}) error {

	self.wlock.Lock()
	defer self.wlock.Unlock()

	self.wbuf.Reset()
	self.wbuf.Write([]byte{'M', 0, 0, 0})

	enc := msgpack.NewEncoder(&self.wbuf)
	if err := enc.Encode(msg); err != nil {
		return err
	}
	if self.wbuf.Len() > 65000 {
		return fmt.Errorf("msg too large")
	}

	var l = uint16(self.wbuf.Len() - 4)
	binary.LittleEndian.PutUint16(self.wbuf.Bytes()[2:], l)

	if _, err := self.writeAll(self.wbuf.Bytes()); err != nil {
		return err
	}

	return nil
}

func (self *Sock) writeAll(b []byte) (int, error) {

	var n int
	for len(b) > 0 {
		self.inner.SetWriteDeadline(time.Now().Add(self.interval * 10))
		n, err := self.inner.Write(b)
		if err != nil {
			if self.err != nil {
				return n, self.err
			}
			return n, err
		}
		b = b[n:]
	}
	return n, nil
}
