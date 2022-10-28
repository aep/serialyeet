yeet
====

yeet a message over a socket.
the most important feature is it does time outs correctly,


### protocol

| byte |                                |
|------|--------------------------------|
| 0    | message type  			|
| 1    | reserved      			|
| 2-3  | little endian content size     |
| ..   | content     			|


### message types


| type |                                  |
|------|----------------------------------|
| a-z  | optional. ignored for future use |
| M    | msgpack content                  |
| P    | ping                             |
| R    | pong                             |
| C    | close                            |
| E    | error message                    |
| H    | handshake (must be first message)|



