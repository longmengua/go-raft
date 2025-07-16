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
		WALDir:         configs.FileDir,     // WAL(日誌寫前日誌)的存放目錄，建議使用低延遲存儲裝置
		NodeHostDir:    configs.FileDir,     // NodeHost 其他資料(快照、狀態等)的存放目錄
		RaftAddress:    configs.RaftAddress, // 本節點 Raft 通訊地址 (IP:Port)
		RTTMillisecond: 200,                 // 節點間平均往返延遲時間 (ms)，用於調整心跳和選舉時間
		Expert: config.ExpertConfig{
			LogDB: config.LogDBConfig{
				Shards:                             8,                 // LogDB shard 數量，提高並行度和吞吐量。增加 shards 數可以讓 LogDB 有更好的並行處理能力，但也會佔用更多資源。一般8-16是合理範圍。
				KVKeepLogFileNum:                   10,                // 保留的舊 log 文件數量，超過刪除回收空間。控制保留多少舊的 WAL 日誌文件，數字大了恢復時間較長但安全性較高，數字小會釋放空間但恢復資料有限。
				KVMaxBackgroundCompactions:         4,                 // 最大後台壓縮併發數，影響壓縮速度和 IO 使用。壓縮是比較耗資源的動作，調大可以提升寫入性能，但也可能影響 IO 延遲。
				KVMaxBackgroundFlushes:             2,                 // 最大後台 flush 併發數，將 MemTable 刷寫到磁碟的併發數。
				KVLRUCacheSize:                     64 * 1024 * 1024,  // LRU 快取大小 (64MB)，加速讀取性能。快取太小會導致讀取頻繁從磁碟載入，太大則佔用過多記憶體。
				KVWriteBufferSize:                  64 * 1024 * 1024,  // MemTable 寫緩衝大小 (64MB)，影響寫入延遲和頻率。影響 MemTable 大小及數量，決定寫入延遲和 flush 頻率。一般 64MB 大小不錯，數量3可避免阻塞。
				KVMaxWriteBufferNumber:             3,                 // 最大同時存在的 MemTable 數量，超過會阻塞寫入。影響 MemTable 大小及數量，決定寫入延遲和 flush 頻率。一般 64MB 大小不錯，數量3可避免阻塞。
				KVLevel0FileNumCompactionTrigger:   4,                 // Level 0 的檔案數超過該值觸發壓縮。
				KVLevel0SlowdownWritesTrigger:      8,                 // Level 0 的檔案數超過該值時減慢寫入速度以防止資源耗盡
				KVLevel0StopWritesTrigger:          12,                // Level 0 的檔案數超過該值時暫停寫入。如果 Level 0 檔案太多會暫停寫入，避免系統崩潰，但可能導致寫入延遲。
				KVMaxBytesForLevelBase:             256 * 1024 * 1024, // Level 1 的最大檔案大小基準 (256MB)
				KVMaxBytesForLevelMultiplier:       10,                // 每個 level 檔案大小相較上一層的倍數
				KVTargetFileSizeBase:               64 * 1024 * 1024,  // 壓縮時的目標檔案大小基準 (64MB)
				KVTargetFileSizeMultiplier:         1,                 // 壓縮檔案大小倍數
				KVLevelCompactionDynamicLevelBytes: 1,                 // 是否啟用動態調整 Level 大小
				KVRecycleLogFileNum:                2,                 // 重複利用的 log 檔案數量，降低文件創建刪除頻率
				KVNumOfLevels:                      7,                 // Level 數量，影響壓縮層次結構深度
				KVBlockSize:                        4096,              // Block 大小 (4KB)，影響 IO 效率和壓縮率。一般 4KB 是典型磁碟IO塊大小，較大會提升IO效率但壓縮率可能下降。
				SaveBufferSize:                     64 * 1024,         // 儲存緩衝大小 (64KB)，寫入時用的中間緩衝
				MaxSaveBufferSize:                  256 * 1024,        // 最大儲存緩衝大小 (256KB)
			},
		},
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
			ElectionRTT:        30,                // 6秒選舉超時 (RTT=200ms)。更長的選舉超時。太小 ➔ 容易腦裂；太大 ➔ Failover 慢
			HeartbeatRTT:       2,                 // 400ms 心跳。保持心跳頻率。太小 ➔ 浪費頻寬；太大 ➔ Failover 變慢
			ReplicaID:          configs.NodeID,    // 本節點的 Raft Replica ID (唯一)
			ShardID:            configs.ClusterID, // 所屬的 Raft 群組 (Shard) ID
			CheckQuorum:        true,              // 保護資料一致性，防止腦裂。
			SnapshotEntries:    200,               // 5萬條快照。太小 ➔ 快照太頻繁；太大 ➔ 重新啟動慢
			CompactionOverhead: 50,                // 2千條保留日誌。調太小會造成 follower 追不上，導致 full snapshot
			MaxInMemLogSize:    4 * 1024 * 1024,   // 4MB。太小會導致頻繁 snapshot & 日誌丟磁碟
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
