package yeet

import (
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

type TestMessage struct {
	Alice string
	Bob   int
}

func TestYeet(t *testing.T) {

	go func() {
		time.Sleep(time.Second)
		panic("should be done by now")
	}()

	//sockA, sockB := pipe()
	sockA, sockB := net.Pipe()
	defer sockA.Close()
	defer sockB.Close()

	go func() {
		yeetA, err := Connect(sockA, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
		if err != nil {
			panic(err)
		}
		go yeetA.Discard()
		err = yeetA.Write(TestMessage{"Alice", 1})
		if err != nil {
			panic(err)
		}
	}()

	yeetB, err := Connect(sockB, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	var msg TestMessage
	err = yeetB.Read(&msg)
	if err != nil {
		t.Error(err)
		return
	}
	if msg.Alice != "Alice" {
		t.Error("mismatched")
		return
	}
	if msg.Bob != 1 {
		t.Error("mismatched")
		return
	}
}

func TestHandshakeTimeout(t *testing.T) {

	go func() {
		time.Sleep(time.Second)
		panic("should have timed out")
	}()

	sockA, _ := net.Pipe()
	_, err := Connect(sockA, HandshakeTimeout(10*time.Millisecond), Keepalive(10*time.Millisecond))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Error("error should be a timeout")
	}
}

func TestReadDeadline(t *testing.T) {

	go func() {
		time.Sleep(time.Second)
		panic("should have timed out")
	}()

	//sockA, sockB := pipe()
	sockA, sockB := net.Pipe()
	defer sockA.Close()
	defer sockB.Close()

	go func() {
		yeetA, err := Connect(sockA, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
		if err != nil {
			panic(err)
		}
		go yeetA.Discard()
		err = yeetA.Write(TestMessage{"Alice", 1})
		if err != nil {
			panic(err)
		}
	}()

	yeetB, err := Connect(sockB, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	var msg TestMessage

	yeetB.Read(&msg)

	yeetB.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
	err = yeetB.Read(&msg)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Error("error should be a timeout")
	}

}

func pipe() (net.Conn, net.Conn) {
	l, err := net.Listen("tcp", "127.0.0.1:12321")
	if err != nil {
		panic(err)
	}

	conA := make(chan net.Conn)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		conA <- conn
	}()

	connB, err := net.Dial("tcp", "127.0.0.1:12321")
	if err != nil {
		panic(err)
	}

	connA := <-conA

	return connA, connB

}

func NewDebugConn() (net.Conn, net.Conn) {
	r1, w1 := net.Pipe()
	r2, w2 := net.Pipe()
	return &DebugConn{r1, w2, ">"}, &DebugConn{r2, w1, "<"}
}

type DebugConn struct {
	r net.Conn
	w net.Conn
	n string
}

func (c *DebugConn) Read(b []byte) (n int, err error) {
	n, err = c.r.Read(b)
	fmt.Println(c.n, " read ", n, " bytes %q : %v", b[:n], err)
	return n, err
}

func (c *DebugConn) Write(b []byte) (n int, err error) {
	n, err = c.w.Write(b)
	fmt.Println(c.n, " written ", n, " bytes %q : %v", b, err)
	return n, err
}

func (c *DebugConn) Close() error {
	c.r.Close()
	c.w.Close()
	return nil
}

func (c *DebugConn) LocalAddr() net.Addr {
	return c.r.LocalAddr()
}

func (c *DebugConn) RemoteAddr() net.Addr {
	return c.r.RemoteAddr()
}

func (c *DebugConn) SetDeadline(t time.Time) error {
	c.r.SetDeadline(t)
	c.w.SetDeadline(t)
	return nil
}

func (c *DebugConn) SetReadDeadline(t time.Time) error {
	c.r.SetReadDeadline(t)
	return nil
}

func (c *DebugConn) SetWriteDeadline(t time.Time) error {
	c.w.SetWriteDeadline(t)
	return nil
}
