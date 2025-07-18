package raft

import (
	"log"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
)

type RaftStore struct {
	NodeHost    *dragonboat.NodeHost
	ClusterID   uint64
	NodeID      uint64
	FileDir     string
	RaftAddress string
}

// Config 定義啟動 NodeHost 的參數
type NodeConfig struct {
	FileDir     string
	RaftAddress string
	NodeID      uint64
	ClusterID   uint64
}

// New 支援傳入多節點參數來建立 RaftStore
//
//	defaultCfg := raft.NodeConfig{
//		FileDir:     configs.FileDir,
//		RaftAddress: configs.RaftAddress,
//		NodeID:      configs.NodeID,
//		ClusterID:   configs.ClusterID,
//	}
func New(cfg NodeConfig) (*RaftStore, error) {
	logger.GetLogger("raft").SetLevel(logger.DEBUG)

	nh, err := dragonboat.NewNodeHost(config.NodeHostConfig{
		WALDir:         cfg.FileDir,
		NodeHostDir:    cfg.FileDir,
		RaftAddress:    cfg.RaftAddress,
		RTTMillisecond: 200,
		Expert: config.ExpertConfig{
			LogDB: config.LogDBConfig{
				Shards:                             8,
				KVKeepLogFileNum:                   10,
				KVMaxBackgroundCompactions:         4,
				KVMaxBackgroundFlushes:             2,
				KVLRUCacheSize:                     64 * 1024 * 1024,
				KVWriteBufferSize:                  64 * 1024 * 1024,
				KVMaxWriteBufferNumber:             3,
				KVLevel0FileNumCompactionTrigger:   4,
				KVLevel0SlowdownWritesTrigger:      8,
				KVLevel0StopWritesTrigger:          12,
				KVMaxBytesForLevelBase:             256 * 1024 * 1024,
				KVMaxBytesForLevelMultiplier:       10,
				KVTargetFileSizeBase:               64 * 1024 * 1024,
				KVTargetFileSizeMultiplier:         1,
				KVLevelCompactionDynamicLevelBytes: 1,
				KVRecycleLogFileNum:                2,
				KVNumOfLevels:                      7,
				KVBlockSize:                        4096,
				SaveBufferSize:                     64 * 1024,
				MaxSaveBufferSize:                  256 * 1024,
			},
		},
	})

	if err != nil {
		return nil, err
	}

	// 多個時候，下面為 for 產生。
	err = nh.StartConcurrentReplica(
		map[uint64]string{cfg.NodeID: cfg.RaftAddress},
		false,
		func(clusterID, nodeID uint64) statemachine.IConcurrentStateMachine {
			return NewAssetRaftConcurrentMachine(clusterID, nodeID)
		},
		config.Config{
			ElectionRTT:        30,
			HeartbeatRTT:       2,
			ReplicaID:          cfg.NodeID,
			ShardID:            cfg.ClusterID,
			CheckQuorum:        true,
			SnapshotEntries:    200,
			CompactionOverhead: 50,
			MaxInMemLogSize:    4 * 1024 * 1024,
		},
	)

	if err != nil {
		return nil, err
	}

	log.Printf("Raft Node Started at %s with cluster %d, node %d\n", cfg.RaftAddress, cfg.ClusterID, cfg.NodeID)

	return &RaftStore{
		NodeHost:    nh,
		ClusterID:   cfg.ClusterID,
		FileDir:     cfg.FileDir,
		NodeID:      cfg.NodeID,
		RaftAddress: cfg.RaftAddress,
	}, nil
}
