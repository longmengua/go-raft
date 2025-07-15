package raft

import (
	"go-raft/internal/configs"
	"log"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
)

type RaftStore struct {
	NodeHost  *dragonboat.NodeHost
	ClusterID uint64
}

func New() (*RaftStore, error) {
	logger.GetLogger("raft").SetLevel(logger.DEBUG)

	nh, err := dragonboat.NewNodeHost(config.NodeHostConfig{
		WALDir:         configs.FileDir,
		NodeHostDir:    configs.FileDir,
		RTTMillisecond: 200,
		RaftAddress:    configs.RaftAddress,
	})
	if err != nil {
		return nil, err
	}

	err = nh.StartConcurrentReplica(
		map[uint64]string{configs.NodeID: configs.RaftAddress},
		false,
		func(clusterID, nodeID uint64) statemachine.IConcurrentStateMachine {
			return NewAssetRaftConcurrentMachine()
		},
		config.Config{
			ReplicaID:          configs.NodeID,
			ShardID:            configs.ClusterID,
			ElectionRTT:        20,   // 更長的選舉超時
			HeartbeatRTT:       1,    // 保持心跳頻率
			CheckQuorum:        true, // 啟用法定人數檢查
			SnapshotEntries:    100,  // 每10000條日誌觸發快照
			CompactionOverhead: 50,   // 保留5000條歷史日誌
			// 可選的高級參數：
			MaxInMemLogSize: 8 * 1024 * 1024, // 內存中日誌最大大小 (8MB)
		},
	)
	if err != nil {
		return nil, err
	}

	log.Printf("Raft Node Started at %s with cluster %d, node %d\n", configs.RaftAddress, configs.ClusterID, configs.NodeID)
	return &RaftStore{
		NodeHost:  nh,
		ClusterID: configs.ClusterID,
	}, nil
}
