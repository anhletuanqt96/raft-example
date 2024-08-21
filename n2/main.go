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
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID("node2")

	dir := "./node2"
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

	transport, err := raft.NewTCPTransport("127.0.0.1:8001", nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("failed to create transport: %v", err)
	}

	fsm := &fsm{}

	r, err = raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		log.Fatalf("failed to create raft: %v", err)
	}

	r.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID("node2"),
				Address: transport.LocalAddr(),
			},
			{
				ID:      raft.ServerID("node1"),
				Address: raft.ServerAddress("127.0.0.1:8000"),
			},
		},
	})

	http.HandleFunc("/apply", applyHandler)
	go http.ListenAndServe(":9001", nil)

	select {}
}

func applyHandler(w http.ResponseWriter, req *http.Request) {
	entry := req.URL.Query().Get("entry")
	if entry == "" {
		http.Error(w, "missing log entry", http.StatusBadRequest)
		return
	}

	f := r.Apply([]byte(entry), 10*time.Second)
	if err := f.Error(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("log entry applied"))
}

type fsm struct{}

func (f *fsm) Apply(l *raft.Log) interface{} {
	log.Printf("Apply log: %v", string(l.Data))
	return nil
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{}, nil
}

func (f *fsm) Restore(rc io.ReadCloser) error {
	return nil
}

type fsmSnapshot struct{}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	sink.Close()
	return nil
}

func (f *fsmSnapshot) Release() {}
