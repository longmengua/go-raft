package snapshot

import (
	"bytes"
	"encoding/gob"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v4"
)

type Handler struct {
	nh        *dragonboat.NodeHost
	clusterID uint64
}

func NewHanlder(nh *dragonboat.NodeHost, clusterID uint64) *Handler {
	return &Handler{nh: nh, clusterID: clusterID}
}

// 假設多個節點的話，可以用 dragonboat client 去通知某個節點，就不用開多個 http server
func (h *Handler) SetSnapshotVersion(c *gin.Context) {
	var req RequestSetSnapshot
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	versionInt, err := strconv.Atoi(req.Version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version number"})
		return
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&versionInt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encode failed"})
		return
	}

	session := h.nh.GetNoOPSession(h.clusterID)
	if _, err := h.nh.SyncPropose(c.Request.Context(), session, buf.Bytes()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "raft propose failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": versionInt})
}

func (h *Handler) GetSnapshotVersion(c *gin.Context) {
	query := "version"
	value, err := h.nh.SyncRead(c.Request.Context(), h.clusterID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GetSnapshotVersion failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": value})
}
