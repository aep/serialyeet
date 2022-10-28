yeet
====

yeet a message over a socket.
the most important feature is it does time outs correctly,


protocol:

	- 1 byte message type
	- 1 byte reserved
	- 2 byte little endian content size
	- content

message types a-z are ignored  but fully read for future use
message type M means msgpack content
message type P means ping, which must be answered with R for pong response
message type E means error
message type C means close
