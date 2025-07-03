package main

import (
	"fmt"
	"go-raft/api"
	"go-raft/raft"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
)

const (
	clusterID   = 100
	nodeID      = 1
	raftAddress = "localhost:5010"
)

func main() {
	logger.GetLogger("raft").SetLevel(logger.WARNING)

	nh, err := dragonboat.NewNodeHost(config.NodeHostConfig{
		WALDir:         raft.FileDir,
		NodeHostDir:    raft.FileDir,
		RTTMillisecond: 200,
		RaftAddress:    raftAddress,
	})
	if err != nil {
		log.Fatalf("failed to create nodehost: %v", err)
	}

	err = nh.StartReplica(
		map[uint64]string{nodeID: raftAddress},
		false,
		func(clusterID, nodeID uint64) statemachine.IStateMachine {
			return raft.NewAssetRaftMachine()
		},
		config.Config{
			ReplicaID: nodeID,
			// NodeID:             nodeID,
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

	fmt.Println("Raft Node Started")

	// 啟動 API
	r := gin.Default()
	handler := api.NewHandler(nh, clusterID)

	r.POST("/asset/add", handler.AddAsset)
	r.GET("/asset/balance", handler.GetBalance)

	r.Run(":8080")
}
