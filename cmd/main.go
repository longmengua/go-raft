package main

import (
	"context"
	"go-raft/api"
	raftconcurrent "go-raft/raft_concurrent"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
)

const (
	clusterID   = 99
	nodeID      = 1
	raftAddress = "localhost:5010"
)

func main() {
	logger.GetLogger("raft").SetLevel(logger.DEBUG)

	nh, err := dragonboat.NewNodeHost(config.NodeHostConfig{
		WALDir:         raftconcurrent.FileDir,
		NodeHostDir:    raftconcurrent.FileDir,
		RTTMillisecond: 200,
		RaftAddress:    raftAddress,
	})
	if err != nil {
		log.Fatalf("failed to create nodehost: %v", err)
	}

	err = nh.StartConcurrentReplica(
		map[uint64]string{nodeID: raftAddress},
		false,
		func(clusterID, nodeID uint64) statemachine.IConcurrentStateMachine {
			return raftconcurrent.NewAssetRaftConcurrentMachine()
		},
		config.Config{
			ReplicaID:          nodeID,
			ShardID:            clusterID,
			ElectionRTT:        10,
			HeartbeatRTT:       1,
			CheckQuorum:        true,
			SnapshotEntries:    10,
			CompactionOverhead: 5,
		},
	)
	if err != nil {
		log.Fatalf("failed to start replica: %v", err)
	}

	log.Printf("Raft Node Started at %s with cluster %d, node %d\n", raftAddress, clusterID, nodeID)

	// 啟動 API
	r := gin.Default()
	r.Use(RequestTimeoutMiddleware(5 * time.Second))

	handler := api.NewHandler(nh, clusterID)

	r.POST("/asset/add", handler.AddAsset)
	r.GET("/asset/balance", handler.GetBalance)
	r.GET("/asset/balances", handler.GetBalances)

	r.Run(":8080")
}

func RequestTimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
