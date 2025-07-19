package raft_test

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"go-raft/internal/configs"
	"go-raft/internal/domain"
	"go-raft/internal/raft"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestRaftRollingUpgradeAndSnapshotSwitch(t *testing.T) {
	clusterID := uint64(101)
	basePort := 24000
	baseDir := "tmp/raft-node"
	host := "localhost"

	var nodes []*raft.RaftStore

	// 準備 InitialMembers（要在 leader 啟動時給）
	initialMembers := map[uint64]string{
		1: fmt.Sprintf("%s:%d", host, basePort),
		2: fmt.Sprintf("%s:%d", host, basePort+1),
		3: fmt.Sprintf("%s:%d", host, basePort+2),
	}

	// Step 1：啟動 Leader（Join=false 並傳 initialMembers）
	leaderNodeID := uint64(1)
	leaderAddr := initialMembers[leaderNodeID]
	configs.SetSnapshotVersion(clusterID, leaderNodeID, 1)

	leader, err := raft.New(raft.NodeConfig{
		FileDir:        fmt.Sprintf("%s-%d", baseDir, leaderNodeID),
		RaftAddress:    leaderAddr,
		NodeID:         leaderNodeID,
		ClusterID:      clusterID,
		Join:           false,
		InitialMembers: initialMembers,
	})
	if err != nil {
		t.Fatalf("Failed to create leader node: %v", err)
	}
	if err := leader.Start(); err != nil {
		t.Fatalf("Failed to start leader node: %v", err)
	}
	waitForShardReady(t, leader, clusterID)
	nodes = append(nodes, leader)

	// 找出 Leader
	leader, err = findLeaderWithRaftStore(nodes, clusterID)
	if err != nil {
		t.Fatal("Leader not found")
	}

	// 寫入前五筆資料
	for i := 0; i < 5; i++ {
		cmd := domain.Asset{
			UID:      fmt.Sprintf("user-%d", i),
			Currency: "USD",
			Amount:   float64(i + 1),
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(cmd); err != nil {
			t.Fatalf("Encode error: %v", err)
		}
		session := leader.NodeHost.GetNoOPSession(clusterID)
		if _, err := leader.NodeHost.SyncPropose(context.Background(), session, buf.Bytes()); err != nil {
			t.Fatalf("Propose failed: %v", err)
		}
	}

	// 等待所有節點同步資料
	for _, node := range nodes {
		waitForCompleteData(t, node, clusterID, 5)
	}

	// 模擬 snapshot version 切換到 v2
	for _, node := range nodes {
		configs.SetSnapshotVersion(clusterID, node.NodeID, 2)
	}

	// 滾動重啟節點
	for i, node := range nodes {
		node.NodeHost.Close()
		time.Sleep(2 * time.Second)

		newNode, err := raft.New(raft.NodeConfig{
			FileDir:     node.FileDir,
			RaftAddress: node.RaftAddress,
			NodeID:      node.NodeID,
			ClusterID:   clusterID,
			Join:        true,
		})
		if err != nil {
			t.Fatalf("Failed to recreate node %d: %v", node.NodeID, err)
		}
		if err := newNode.Start(); err != nil {
			t.Fatalf("Failed to restart node %d: %v", node.NodeID, err)
		}
		waitForShardReady(t, newNode, clusterID)
		nodes[i] = newNode
	}

	// 再寫入五筆資料
	for i := 5; i < 10; i++ {
		cmd := domain.Asset{
			UID:      fmt.Sprintf("user-%d", i),
			Currency: "USD",
			Amount:   float64(i + 1),
		}
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(cmd); err != nil {
			t.Fatalf("Encode error: %v", err)
		}
		session := leader.NodeHost.GetNoOPSession(clusterID)
		if _, err := leader.NodeHost.SyncPropose(context.Background(), session, buf.Bytes()); err != nil {
			t.Fatalf("Propose failed: %v", err)
		}
	}

	// 確認資料同步完成
	for _, node := range nodes {
		waitForCompleteData(t, node, clusterID, 10)
	}
}

func waitForShardReady(t *testing.T, rs *raft.RaftStore, clusterID uint64) {
	const maxAttempts = 10
	const interval = time.Second * 2

	for range maxAttempts {
		leaderID, _, valid, err := rs.NodeHost.GetLeaderID(clusterID)
		if err != nil {
			logrus.WithError(err).WithField("nodeID", rs.NodeID).Info("GetLeaderID error, shard may not ready yet")
		} else if valid {
			logrus.WithFields(logrus.Fields{
				"nodeID":   rs.NodeID,
				"leaderID": leaderID,
			}).Info("Shard ready with leader")
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("Node %d shard not ready after %d attempts", rs.NodeID, maxAttempts)
}

func waitForCompleteData(t *testing.T, rs *raft.RaftStore, clusterID uint64, expectedCount int) {
	const maxAttempts = 10
	const interval = time.Second

	for range maxAttempts {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := rs.NodeHost.SyncRead(ctx, clusterID, "list")
		cancel()
		if err != nil {
			logrus.WithError(err).WithField("nodeID", rs.NodeID).Warn("SyncRead failed")
		} else {
			assetsMap, ok := result.(map[string]map[string]float64)
			if !ok {
				t.Fatalf("Node %d unexpected data type %T", rs.NodeID, result)
			}
			total := 0
			for _, currencyMap := range assetsMap {
				total += len(currencyMap)
			}
			if total == expectedCount {
				logrus.WithField("nodeID", rs.NodeID).Info("Data synchronized successfully")
				return
			}
		}
		time.Sleep(interval)
	}
	t.Fatalf("Node %d data sync timeout — expected %d items", rs.NodeID, expectedCount)
}

func findLeaderWithRaftStore(nodes []*raft.RaftStore, clusterID uint64) (*raft.RaftStore, error) {
	const maxAttempts = 10
	const interval = time.Second

	for range maxAttempts {
		for _, n := range nodes {
			leaderID, _, valid, err := n.NodeHost.GetLeaderID(clusterID)
			if err == nil && valid && leaderID == n.NodeID {
				return n, nil
			}
		}
		time.Sleep(interval)
	}
	return nil, fmt.Errorf("no leader found within %d attempts", maxAttempts)
}
