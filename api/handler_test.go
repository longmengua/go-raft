package api_test

import (
	"bytes"
	"encoding/json"
	"go-raft/api"
	"go-raft/raft"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/statemachine"
	"github.com/stretchr/testify/assert"
)

const (
	clusterID   = 100
	nodeID      = 1
	raftAddress = "localhost:5010"
)

func setupTestHandler(t *testing.T) *api.Handler {
	t.Helper()

	nh, err := dragonboat.NewNodeHost(config.NodeHostConfig{
		WALDir:         raft.FileDir,
		NodeHostDir:    raft.FileDir,
		RTTMillisecond: 200,
		RaftAddress:    raftAddress,
	})
	assert.NoError(t, err)

	err = nh.StartReplica(
		map[uint64]string{nodeID: raftAddress},
		false, // join = false for initial start
		func(clusterID, nodeID uint64) statemachine.IStateMachine {
			return raft.NewAssetRaftMachine()
		},
		config.Config{
			ReplicaID:          nodeID,
			ElectionRTT:        10,
			HeartbeatRTT:       1,
			CheckQuorum:        true,
			SnapshotEntries:    10,
			CompactionOverhead: 5,
		},
	)
	assert.NoError(t, err)

	// 等待選舉完成
	time.Sleep(1 * time.Second)

	return api.NewHandler(nh, clusterID)
}

func TestAddAsset_Success(t *testing.T) {
	handler := setupTestHandler(t)
	router := gin.Default()
	router.POST("/asset/add", handler.AddAsset)

	reqBody := map[string]interface{}{
		"uid":      "testuser",
		"currency": "BTC",
		"amount":   0.25,
	}
	jsonData, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/asset/add", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"asset updated"`)
}

func TestGetBalance_Success(t *testing.T) {
	handler := setupTestHandler(t)
	router := gin.Default()
	router.POST("/asset/add", handler.AddAsset)
	router.GET("/asset/balance", handler.GetBalance)

	// 先新增資產
	addBody := map[string]interface{}{
		"uid":      "testuser",
		"currency": "ETH",
		"amount":   1.5,
	}
	jsonData, _ := json.Marshal(addBody)

	addReq, _ := http.NewRequest("POST", "/asset/add", bytes.NewReader(jsonData))
	addReq.Header.Set("Content-Type", "application/json")
	addW := httptest.NewRecorder()
	router.ServeHTTP(addW, addReq)
	assert.Equal(t, http.StatusOK, addW.Code)

	// 查詢資產
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/asset/balance?uid=testuser&currency=ETH", nil)

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ETH", resp["currency"])
	assert.Equal(t, "testuser", resp["uid"])
	assert.Equal(t, 1.5, resp["balance"])
}

func TestAddAsset_InvalidInput(t *testing.T) {
	handler := setupTestHandler(t)
	router := gin.Default()
	router.POST("/asset/add", handler.AddAsset)

	// 少了 amount
	reqBody := map[string]interface{}{
		"uid":      "u1",
		"currency": "BTC",
	}
	jsonData, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/asset/add", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`)
}

func TestGetBalance_InvalidQuery(t *testing.T) {
	handler := setupTestHandler(t)
	router := gin.Default()
	router.GET("/asset/balance", handler.GetBalance)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/asset/balance", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`)
}
