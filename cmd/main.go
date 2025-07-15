package main

import (
	"context"
	"go-raft/internal/adapters/http"
	"go-raft/internal/adapters/http/asset"
	"go-raft/internal/raft"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize the Raft store
	raftstore, err := raft.New()
	if err != nil {
		log.Fatalf("failed to start replica: %v", err)
	}

	// Initialize all hanlders
	assethandler := asset.NewHanlder(raftstore.NodeHost, raftstore.ClusterID)

	// [::1]:19090 for ipv6
	httpserver := http.New([]string{"0.0.0.0:9090"}, assethandler)
	go func() {
		if err := httpserver.Start(); err != nil {
			log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()

	// 等待中斷訊號
	<-ctx.Done()
	log.Println("Main: shutdown signal received")

	// todo: 放所有需要graceful shutdown 的函式

	log.Println("Main: all servers shutdown cleanly")
}
