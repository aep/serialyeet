package yeet

import (
	"net"
	"testing"
	"time"
)

type TestMessage struct {
	Alice string
	Bob   int
}

func TestYeet(t *testing.T) {

	sockA, sockB := net.Pipe()
	yeetA := New(sockA)
	yeetB := New(sockB)

	go func() {
		err := yeetA.Write(TestMessage{"Alice", 1})
		if err != nil {
			t.Error(err)
		}
	}()

	var msg TestMessage
	err := yeetB.Read(&msg)
	if err != nil {
		t.Error(err)
	}
	if msg.Alice != "Alice" {
		t.Error("mismatched")
	}
	if msg.Bob != 1 {
		t.Error("mismatched")
	}
}

func TestWriteTimeout(t *testing.T) {

	sockA, _ := net.Pipe()
	yeetA := New(sockA, Keepalive(time.Millisecond*10))

	go func() {
		time.Sleep(time.Millisecond * 30)
		panic("should have timed out")
	}()

	yeetA.Write(TestMessage{"Alice", 1})

}

func TestReadTimeout(t *testing.T) {

	sockA, _ := net.Pipe()
	yeetA := New(sockA, Keepalive(time.Millisecond*10))

	go func() {
		time.Sleep(time.Millisecond * 30)
		panic("should have timed out")
	}()

	var msg TestMessage
	yeetA.Read(&msg)
}
