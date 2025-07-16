package configs

const (
	// public
	FileDir = "raft-snapshots"

	// private
	ClusterID   = 99
	NodeID      = 1
	RaftAddress = "localhost:5010"
)

// 能交由 API 管理，為了rolling update 不同資料結構用。
var SnapshotVersion = 1
