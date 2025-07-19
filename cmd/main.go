package main

import (
	"context"
	"fmt"
	"go-raft/internal/adapters/http"
	"go-raft/internal/adapters/http/asset"
	"go-raft/internal/configs"
	"go-raft/internal/raft"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := configs.ParseConfigFile("config.yaml")
	if err != nil {
		fmt.Println("解析 config 失敗:", err)
		return
	}

	// Initialize the Raft store
	raftstore := raft.New(&cfg.Raft, &cfg.Shards)
	err = raftstore.Start()
	if err != nil {
		log.Fatalf("failed to start replica: %v", err)
	}

	// Initialize all hanlders
	assethandler := asset.NewHanlder(raftstore.NodeHost, raftstore.RaftCfg.ClusterID)

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
