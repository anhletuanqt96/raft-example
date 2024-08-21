# How to run

- `cd n1` and then `go run main.go`
- `cd n2` and then `go run main.go`
- `curl "http://127.0.0.1:9000/apply?entry=HelloRaft"` or `curl "http://127.0.0.1:9001/apply?entry=HelloRaft"` depends on which node is leadership
  => logs will appear on 2 nodes
