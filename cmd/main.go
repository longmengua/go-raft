package main

import (
	"go-raft/internal/adapters/http"
	"go-raft/internal/adapters/http/asset"
	"go-raft/internal/raft"
	"log"
)

func main() {
	// Initialize the Raft store
	raftstore, err := raft.New()
	if err != nil {
		log.Fatalf("failed to start replica: %v", err)
	}

	// Initialize all hanlders
	assethandler := asset.NewHanlder(raftstore.NodeHost, raftstore.ClusterID)

	// [::1]:19090 for ipv6
	httpserver := http.New([]string{":19090", "0.0.0.0:9090", "[::1]:9090"}, assethandler)
	go func() {
		if err := httpserver.Start(); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()
}
