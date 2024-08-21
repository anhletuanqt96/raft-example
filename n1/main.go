package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

var r *raft.Raft

func main() {
	// create and config node
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID("node1")

	// create storage for log and snapshot
	dir := "./node1"
	os.MkdirAll(dir, 0700)
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dir, "raft-log.bolt"))
	if err != nil {
		log.Fatalf("failed to create log store: %v", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dir, "raft-stable.bolt"))
	if err != nil {
		log.Fatalf("failed to create stable store: %v", err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(dir, 1, os.Stderr)
	if err != nil {
		log.Fatalf("failed to create snapshot store: %v", err)
	}

	// config transport for Raft node
	transport, err := raft.NewTCPTransport("127.0.0.1:8000", nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("failed to create transport: %v", err)
	}

	// create a fake FSM (Finite State Machine) to manage log
	fsm := &fsm{}

	// init Raft
	r, err = raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		log.Fatalf("failed to create raft: %v", err)
	}

	// add nodes to Raft cluster
	r.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID("node1"),
				Address: transport.LocalAddr(),
			},
			{
				ID:      raft.ServerID("node2"),
				Address: raft.ServerAddress("127.0.0.1:8001"),
			},
		},
	})

	http.HandleFunc("/apply", applyHandler)
	go http.ListenAndServe(":9000", nil)

	select {}
}

// curl "http://127.0.0.1:9000/apply?entry=HelloRaft"
func applyHandler(w http.ResponseWriter, req *http.Request) {
	entry := req.URL.Query().Get("entry")
	if entry == "" {
		http.Error(w, "missing log entry", http.StatusBadRequest)
		return
	}

	// create log entry and apply into Raft
	f := r.Apply([]byte(entry), 10*time.Second)
	if err := f.Error(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("log entry applied"))
}

// FSM is a simple Finite State Machine
type fsm struct{}

// Apply is where log entries are processed.
func (f *fsm) Apply(l *raft.Log) interface{} {
	log.Printf("Apply log: %v", string(l.Data))
	return nil
}

// Snapshot to create snapshot of FSM
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{}, nil
}

// Restore is to restore FSM from snapshot
func (f *fsm) Restore(rc io.ReadCloser) error {
	return nil
}

// fsmSnapshot is a fake snapshot
type fsmSnapshot struct{}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	sink.Close()
	return nil
}

func (f *fsmSnapshot) Release() {}
