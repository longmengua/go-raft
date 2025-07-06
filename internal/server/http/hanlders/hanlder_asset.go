package handlers

import (
	"bytes"
	"encoding/gob"
	"go-raft/internal/domain/modal"
	raftmodal "go-raft/pkg/raft/raft_modal"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v4"
)

type Asset struct {
	nh        *dragonboat.NodeHost
	clusterID uint64
}

func NewHandlerAsset(nh *dragonboat.NodeHost, clusterID uint64) *Asset {
	return &Asset{nh: nh, clusterID: clusterID}
}

func (h *Asset) AddAsset(c *gin.Context) {
	var req modal.RequestAdd
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd := raftmodal.Asset(req)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encode failed"})
		return
	}
	session := h.nh.GetNoOPSession(h.clusterID)
	log.Printf("AddAsset: %d, %+v", h.clusterID, session)
	if _, err := h.nh.SyncPropose(c.Request.Context(), session, buf.Bytes()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "raft propose failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "asset updated"})
}

func (h *Asset) GetBalance(c *gin.Context) {
	uid := c.Query("uid")
	currency := c.Query("currency")
	if uid == "" || currency == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid and currency required"})
		return
	}

	query := raftmodal.Asset{
		UID:      uid,
		Currency: currency,
	}
	value, err := h.nh.SyncRead(c.Request.Context(), h.clusterID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "raft read failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uid":      uid,
		"currency": currency,
		"balance":  value.(float64),
	})
}

func (h *Asset) GetBalances(c *gin.Context) {
	result, err := h.nh.SyncRead(c.Request.Context(), h.clusterID, "list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "raft read failed: " + err.Error()})
		return
	}

	// 修改為正確的 float64 類型
	data, ok := result.(map[string]map[string]float64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid data format from raft"})
		return
	}
	// log.Printf("GetBalances: %d, result: %+v, data: %+v", h.clusterID, result, data)
	c.JSON(http.StatusOK, gin.H{"data": data})
}
