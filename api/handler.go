package api

import (
	"bytes"
	"encoding/gob"
	"go-raft/storage"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v4"
)

type Handler struct {
	nh        *dragonboat.NodeHost
	clusterID uint64
}

func NewHandler(nh *dragonboat.NodeHost, clusterID uint64) *Handler {
	return &Handler{nh: nh, clusterID: clusterID}
}

type AddRequest struct {
	UID      string  `json:"uid" binding:"required"`
	Currency string  `json:"currency" binding:"required"`
	Amount   float64 `json:"amount" binding:"required"`
}

func (h *Handler) AddAsset(c *gin.Context) {
	var req AddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd := storage.AssetCommand{
		UID:      req.UID,
		Currency: req.Currency,
		Amount:   req.Amount,
	}
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

func (h *Handler) GetBalance(c *gin.Context) {
	uid := c.Query("uid")
	currency := c.Query("currency")
	if uid == "" || currency == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid and currency required"})
		return
	}

	query := storage.AssetCommand{
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

func (h *Handler) GetBalances(c *gin.Context) {
	result, err := h.nh.SyncRead(c.Request.Context(), h.clusterID, "list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "raft read failed: " + err.Error()})
		return
	}

	// 轉型成 map[string]map[string]int64
	data, ok := result.(map[string]map[string]int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid data format from raft"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}
