package raft_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"go-raft/internal/domain"
	"go-raft/internal/raft"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestRollingUpgradeWithRaftStore(t *testing.T) {
	clusterID := uint64(1000)
	nodes := []struct {
		dir    string
		addr   string
		nodeID uint64
	}{
		{"./data/node1", "127.0.0.1:8101", 1},
		{"./data/node2", "127.0.0.1:8102", 2},
		{"./data/node3", "127.0.0.1:8103", 3},
	}

	var raftStores []*raft.RaftStore
	// 測試結束時關閉所有 NodeHost，確保資源釋放
	defer func() {
		for _, rs := range raftStores {
			rs.NodeHost.Close()
		}
	}()

	// 建立多個 Raft 節點
	for _, n := range nodes {
		rs, err := raft.New(raft.NodeConfig{
			FileDir:     n.dir,
			RaftAddress: n.addr,
			NodeID:      n.nodeID,
			ClusterID:   clusterID,
		})
		if err != nil {
			t.Fatalf("Failed to start node %d: %v", n.nodeID, err)
		}
		raftStores = append(raftStores, rs)
	}

	// 等待所有節點的分片準備完成（確保 shard ready）
	for _, rs := range raftStores {
		waitForShardReady(t, rs, clusterID)
	}

	// 找出 Leader 節點
	leader, err := findLeaderWithRaftStore(raftStores, clusterID)
	if err != nil {
		t.Fatal("Leader not found:", err)
	}
	t.Logf("Leader is NodeID %d", leader.NodeID)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	sess, err := leader.NodeHost.SyncGetSession(ctx, clusterID)
	defer cancel()
	if err != nil {
		t.Fatal(err)
	}

	// 送出多筆資料提案
	const count = 5
	for i := range count {
		cmd := domain.Asset{
			UID:      fmt.Sprintf("user%d", i),
			Currency: "USD",
			Amount:   float64(i),
		}
		buf := new(bytes.Buffer)
		if err := gob.NewEncoder(buf).Encode(cmd); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err := leader.NodeHost.SyncPropose(ctx, sess, buf.Bytes())
		cancel()
		if err != nil {
			t.Fatalf("Propose failed: %v", err)
		}
	}

	// 滾動重啟每個節點
	// for i, rs := range raftStores {
	// 	rs.NodeHost.Close()
	// 	time.Sleep(10 * time.Second) // 等待資源釋放
	// 	newRS, err := raft.NewWithConfig(raft.NodeConfig{
	// 		FileDir:     rs.FileDir,
	// 		RaftAddress: rs.RaftAddress,
	// 		NodeID:      rs.NodeID,
	// 		ClusterID:   rs.ClusterID,
	// 	})
	// 	if err != nil {
	// 		t.Fatalf("Failed to restart node %d: %v", rs.NodeID, err)
	// 	}
	// 	raftStores[i] = newRS
	// }

	// 再次等待所有節點的分片準備完成
	// for _, rs := range raftStores {
	// 	waitForShardReady(t, rs, clusterID)
	// }

	// // 驗證每個節點資料完整性，使用輪詢等待資料同步完成
	for _, rs := range raftStores {
		waitForCompleteData(t, rs, clusterID, count)
	}
}

// 等待指定節點的 shard 準備就緒，否則持續等待
func waitForShardReady(t *testing.T, rs *raft.RaftStore, clusterID uint64) {
	tick := time.Tick(300 * time.Millisecond)
	var i = 0 // 失敗10次就泡錯誤

	for {
		select {
		case <-tick:
			leaderID, _, valid, err := rs.NodeHost.GetLeaderID(clusterID)
			if err != nil {
				// 加強錯誤日誌，方便診斷
				logrus.WithError(err).WithField("nodeID", rs.NodeID).Info("GetLeaderID error, shard may not ready yet")
				continue
			}
			if valid {
				logrus.WithFields(logrus.Fields{
					"nodeID":   rs.NodeID,
					"leaderID": leaderID,
				}).Info("Shard ready with leader")
				return
			}
			logrus.WithFields(logrus.Fields{
				"nodeID":   rs.NodeID,
				"leaderID": leaderID,
			}).Info("Shard not ready, leader not valid yet")
		default:
			if i > 10 {
				t.Fatalf("Node %d has incomplete data after timeout", rs.NodeID)
			}
			time.Sleep(1 * time.Second)
			i++
			continue
		}
	}
}

func waitForCompleteData(t *testing.T, rs *raft.RaftStore, clusterID uint64, expectedCount int) {
	timeout := time.After(10 * time.Second)
	tick := time.Tick(500 * time.Millisecond)

	for {
		select {
		case <-timeout:
			t.Fatalf("Node %d data sync timeout — expected %d items", rs.NodeID, expectedCount)
		case <-tick:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			result, err := rs.NodeHost.SyncRead(ctx, clusterID, "list")
			cancel()
			if err != nil {
				logrus.WithError(err).WithField("nodeID", rs.NodeID).Warn("SyncRead failed")
				continue
			}

			assetsMap, ok := result.(map[string]map[string]float64)
			if !ok {
				t.Fatalf("Node %d unexpected data type %T", rs.NodeID, result)
			}

			// 計算總筆數
			total := 0
			for _, currencyMap := range assetsMap {
				total += len(currencyMap)
			}
			if total == expectedCount {
				logrus.WithFields(logrus.Fields{
					"nodeID": rs.NodeID,
					"count":  total,
				}).Info("Data synchronized successfully")
				return
			}

			logrus.WithFields(logrus.Fields{
				"nodeID": rs.NodeID,
				"count":  total,
			}).Info("Waiting for data synchronization")
		}
	}
}

// 使用 GetLeaderID 透過節點清單找出 Leader 節點
func findLeaderWithRaftStore(nodes []*raft.RaftStore, clusterID uint64) (*raft.RaftStore, error) {
	timeout := time.After(10 * time.Second)
	tick := time.Tick(500 * time.Millisecond)

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("no leader found within timeout")
		case <-tick:
			for _, n := range nodes {
				leaderID, _, valid, err := n.NodeHost.GetLeaderID(clusterID)
				if err != nil {
					continue
				}
				if valid && leaderID == n.NodeID {
					return n, nil
				}
			}
		}
	}
}
