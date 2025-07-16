package snapshot

import (
	"go-raft/internal/configs"
	"net/http"
	"strconv"

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

	versionInt, err := strconv.Atoi(req.Version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version number"})
		return
	}

	configs.SnapshotVersion = versionInt
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": versionInt})
}

func (h *Handler) GetSnapshotVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": configs.SnapshotVersion})
}
