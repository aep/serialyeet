yeet
====

message passing for distributed system

- aggressive timeouts for liveness detection
- zero allocations


### protocol

| len |                                |
|-----|--------------------------------|
| 4   | little endian key              |
| 1   | ignored for future use         |
| 1   | user flags                     |
| 2   | little endian value size       |
| ..  | value                          |


### reserved keys

| key   |                                  |
|-------|----------------------------------|
| 0     | invalid                          |
| 1     | hello							   |
| 2     | ping                             |
| 3     | pong                             |
| 4     | close                            |
| ..10  | invalid for future use           |
| ..255 | ignored for future use           |



