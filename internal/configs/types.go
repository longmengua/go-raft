package configs

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`

	Raft RaftConfig `yaml:"raft"`

	Shards map[uint64]map[uint64]string `yaml:"shards"` // clusterID => nodeID => raftAddress

	FSM struct {
		Version int `yaml:"version"`
	} `yaml:"fsm"`
}

type RaftConfig struct {
	WALDir      string `yaml:"wal_dir"`
	NodeHostDir string `yaml:"node_host_dir"`
	NodeID      uint64 `yaml:"node_id"`
	ClusterID   uint64 `yaml:"cluster_id"`
	RaftAddress string `yaml:"raft_address"`
}
