package yeet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/vmihailenco/msgpack/v5"
	"io"
	"net"
	"sync"
	"time"
)

type Sock struct {
	inner    net.Conn
	ping     *time.Ticker
	interval time.Duration
	lastSeen time.Time
	err      error

	ctx    context.Context
	cancel context.CancelFunc

	rbuf  bytes.Buffer
	rlock sync.Mutex

	wbuf  bytes.Buffer
	wlock sync.Mutex
}

type SockOpt func(*Sock)

func Keepalive(keepalive time.Duration) SockOpt {
	return func(self *Sock) {
		self.interval = keepalive
	}
}

func New(inner net.Conn, opts ...SockOpt) *Sock {

	ctx, cancel := context.WithCancel(context.Background())
	self := &Sock{
		inner:    inner,
		ctx:      ctx,
		cancel:   cancel,
		interval: time.Second,
	}

	for _, opt := range opts {
		opt(self)
	}

	self.ping = time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				self.ping.Stop()
				return
			case <-self.ping.C:

				self.rlock.Lock()
				var lastSeen = self.lastSeen
				self.rlock.Unlock()

				if lastSeen.Add(self.interval * 2).Before(time.Now()) {
					self.err = fmt.Errorf("keepalive timeout")
					self.Close()
					return
				}

				self.wlock.Lock()
				self.inner.SetWriteDeadline(time.Now().Add(self.interval))
				self.inner.Write([]byte{'P', 0, 0, 0})
				self.wlock.Unlock()
			}
		}
	}()

	return self
}

func (self *Sock) Close() {
	self.wlock.Lock()
	self.inner.Write([]byte{'C', 0, 0, 0})
	defer self.wlock.Unlock()
	self.cancel()
	self.ping.Stop()
	self.inner.Close()
}

func (self *Sock) Read(msg interface{}) error {

	for {

		var header [2]byte
		self.inner.SetReadDeadline(time.Now().Add(self.interval))
		if _, err := io.ReadFull(self.inner, header[:]); err != nil {
			if self.err != nil {
				return self.err
			}
			return fmt.Errorf("failed to read header: %w", err)
		}

		var l uint16
		self.inner.SetReadDeadline(time.Now().Add(self.interval))
		if err := binary.Read(self.inner, binary.LittleEndian, &l); err != nil {
			if self.err != nil {
				return self.err
			}
			return fmt.Errorf("failed to read length: %w", err)
		}

		self.rlock.Lock()
		self.rbuf.Reset()
		self.rbuf.Grow(int(l))

		self.inner.SetReadDeadline(time.Now().Add(self.interval))
		if _, err := io.ReadFull(self.inner, self.rbuf.Bytes()[:l]); err != nil {
			self.rlock.Unlock()
			if self.err != nil {
				return self.err
			}
			return fmt.Errorf("failed to read body: %w", err)
		}

		switch kind := header[0]; {
		case kind > 'a' && kind < 'z':

			self.rlock.Unlock()
			continue

		case kind == 'M':

			err := msgpack.Unmarshal(self.rbuf.Bytes()[:l], msg)
			self.rlock.Unlock()
			if err != nil {
				if self.err != nil {
					return self.err
				}
				return fmt.Errorf("failed to decode msgpack (size %d): %w", l, err)
			} else {
				return nil
			}

		case kind == 'E':

			defer self.rlock.Unlock()
			return fmt.Errorf("remote error: %s", self.rbuf.String())

		case kind == 'C':

			defer self.rlock.Unlock()
			return io.EOF

		case kind == 'P':

			self.rlock.Unlock()

			self.wlock.Lock()
			self.inner.Write([]byte{'R', 0, 0, 0})
			self.wlock.Unlock()

			continue

		case kind == 'R':

			self.rlock.Unlock()
			continue

		default:

			self.rlock.Unlock()
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

	self.inner.SetWriteDeadline(time.Now().Add(self.interval))
	if _, err := self.inner.Write(self.wbuf.Bytes()); err != nil {
		return err
	}

	return nil
}
