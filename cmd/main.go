package main

import (
	"go-raft/internal/server/http"
	handlers "go-raft/internal/server/http/hanlders"
	"go-raft/pkg/raft"
	"log"
)

func main() {
	// Initialize the Raft store
	raftstore, err := raft.New()
	if err != nil {
		log.Fatalf("failed to start replica: %v", err)
	}

	// Initialize all hanlders
	assethandler := handlers.NewHandlerAsset(raftstore.NodeHost, raftstore.ClusterID)

	// [::1]:19090 for ipv6
	httpserver := http.New([]string{":19090", "0.0.0.0:9090", "[::1]:9090"}, assethandler)
	go func() {
		if err := httpserver.Start(); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()
}
