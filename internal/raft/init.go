package raft

import (
	"log"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	"github.com/lni/dragonboat/v4/statemachine"
	"github.com/sirupsen/logrus"
)

type RaftStore struct {
	NodeHost       *dragonboat.NodeHost // Dragonboat 實例，管理 Raft 節點與集群
	NodeID         uint64               // 本節點 ID
	ClusterID      uint64               // 集群 ID
	FileDir        string               // 紀錄檔案與 WAL 儲存的路徑
	RaftAddress    string               // Raft 傳輸的位址
	Join           bool                 //
	initialMembers map[uint64]string
}

// Config 定義啟動 NodeHost 的參數
// 用來初始化 RaftStore 所需的設定值
type NodeConfig struct {
	FileDir        string // 紀錄檔案與 WAL 儲存的目錄
	RaftAddress    string // Raft 傳輸的位址 (host:port)
	NodeID         uint64 // 節點 ID
	ClusterID      uint64 // 集群 ID
	Join           bool   //
	InitialMembers map[uint64]string
}

// New 建立 RaftStore 實例，支援多節點參數傳入
//
// 例如：
//
//	defaultCfg := raft.NodeConfig{
//		FileDir:     configs.FileDir,
//		RaftAddress: configs.RaftAddress,
//		NodeID:      configs.NodeID,
//		ClusterID:   configs.ClusterID,
//	}
func New(nc NodeConfig) (*RaftStore, error) {
	logger.GetLogger("raft").SetLevel(logger.DEBUG)

	nh, err := dragonboat.NewNodeHost(config.NodeHostConfig{
		WALDir:         nc.FileDir,
		NodeHostDir:    nc.FileDir,
		RaftAddress:    nc.RaftAddress,
		RTTMillisecond: 200,
		Expert:         config.ExpertConfig{ /*你的設定*/ },
	})
	if err != nil {
		return nil, err
	}

	log.Printf("NodeHost started at %s (NodeID %d, ClusterID %d)\n", nc.RaftAddress, nc.NodeID, nc.ClusterID)

	return &RaftStore{
		NodeHost:       nh,
		FileDir:        nc.FileDir,
		RaftAddress:    nc.RaftAddress,
		NodeID:         nc.NodeID,
		ClusterID:      nc.ClusterID,
		Join:           nc.Join,
		initialMembers: nc.InitialMembers,
	}, nil
}

func (rs *RaftStore) Start() error {
	logrus.WithFields(logrus.Fields{"Join": rs.Join}).Info("Start")

	var initialMembers map[uint64]string
	if !rs.Join {
		// 只有 Leader 需要 InitialMembers
		initialMembers = rs.initialMembers // 從 RaftStore 中取得
	} else {
		initialMembers = nil // Join 模式不需要
	}

	return rs.NodeHost.StartConcurrentReplica(
		initialMembers, // ✅ 正確傳入 cluster 成員
		rs.Join,
		func(clusterID, nodeID uint64) statemachine.IConcurrentStateMachine {
			return NewAssetRaftConcurrentMachine(clusterID, nodeID)
		},
		config.Config{
			ElectionRTT:        10,
			HeartbeatRTT:       1,
			ReplicaID:          rs.NodeID,
			ShardID:            rs.ClusterID,
			CheckQuorum:        true,
			SnapshotEntries:    10,
			CompactionOverhead: 5,
		},
	)
}
