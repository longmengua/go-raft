package raft

import (
	"fmt"
	"go-raft/internal/configs"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
)

type RaftStore struct {
	NodeHost  *dragonboat.NodeHost
	RaftCfg   *configs.RaftConfig
	ShardsCfg *map[uint64]map[uint64]string
}

func New(
	RaftCfg *configs.RaftConfig,
	ShardsCfg *map[uint64]map[uint64]string,
) *RaftStore {
	logger.GetLogger("raft").SetLevel(logger.INFO)

	nh, err := dragonboat.NewNodeHost(config.NodeHostConfig{
		WALDir:         RaftCfg.WALDir,
		NodeHostDir:    RaftCfg.NodeHostDir,
		RaftAddress:    RaftCfg.RaftAddress,
		RTTMillisecond: 100,
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
		panic(err)
	}

	return &RaftStore{NodeHost: nh, RaftCfg: RaftCfg, ShardsCfg: ShardsCfg}
}

func (rs *RaftStore) Start() error {
	if rs.ShardsCfg == nil {
		panic("RaftStore.Start.ShardsCfg is nil!")
	}

	for shardID, peers := range *rs.ShardsCfg {
		// 只處理包含自己的 replica
		addr, ok := peers[rs.RaftCfg.NodeID]
		if !ok || addr != rs.RaftCfg.RaftAddress {
			continue
		}

		// 啟動前檢查是否已存在
		if rs.NodeHost.HasNodeInfo(shardID, rs.RaftCfg.NodeID) {
			fmt.Printf("[Raft] Shard %d replica %d already started, skip\n", shardID, rs.RaftCfg.NodeID)
			continue
		}

		// 啟動 replica
		err := rs.NodeHost.StartConcurrentReplica(
			peers,
			false,
			func(clusterID, nodeID uint64) statemachine.IConcurrentStateMachine {
				return NewAssetRaftConcurrentMachine()
			},
			config.Config{
				ElectionRTT:        30,
				HeartbeatRTT:       2,
				ReplicaID:          rs.RaftCfg.NodeID,
				ShardID:            shardID,
				CheckQuorum:        true,
				SnapshotEntries:    200,
				CompactionOverhead: 50,
				MaxInMemLogSize:    4 * 1024 * 1024,
			},
		)
		if err != nil {
			return fmt.Errorf("start replica failed for shard %d: %w", shardID, err)
		}
		fmt.Printf("[Raft] Shard %d replica %d started successfully\n", shardID, rs.RaftCfg.NodeID)
	}

	return nil
}
