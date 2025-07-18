package snapshot

type RequestSetSnapshot struct {
	Version uint64 `json:"version" binding:"required"`
	NodeID  uint64 `json:"nodeID" binding:"required"`
	ShardID uint64 `json:"ShardID" binding:"required"`
}
