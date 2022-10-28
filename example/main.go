package main

import (
	"fmt"
	"github.com/aep/yeet"
	"net"
)

func main() {

	server, err := net.Listen("tcp", ":7080")
	if err != nil {
		panic(err)
	}

	go func() {
		conn, err := server.Accept()
		if err != nil {
			panic(err)
		}
		yc, err := yeet.Connect(conn)
		if err != nil {
			panic(err)
		}

		fmt.Println("remote hello:", yc.RemoteHello())

		for {
			msg, err := yc.Read()
			if err != nil {
				panic(fmt.Sprintf("server: %v", err))
			}
			yc.Write(msg)
		}
	}()

	conn, err := net.Dial("tcp", "127.0.0.1:7080")
	if err != nil {
		panic(err)
	}

	yc, err := yeet.Connect(conn, yeet.Hello("hello from client"))
	defer yc.Close()
	yc.Write(yeet.Message{Key: 0xa11c3, Value: []byte("alice")})
	for {
		msg, err := yc.Read()
		if err != nil {
			panic(err)
		}
		println(string(msg.Value))
	}

}
