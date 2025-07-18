package snapshot

import (
	"go-raft/internal/configs"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
}

func NewHanlder() *Handler {
	return &Handler{}
}

func (h *Handler) SetSnapshotVersion(c *gin.Context) {
	var req RequestSetSnapshot
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configs.SetSnapshotVersion(req.NodeID, req.ShardID, req.Version)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "versionInfo": req})
}

func (h *Handler) GetSnapshotVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": configs.GetSnapshotVersions()})
}
