package configs

import "fmt"

var Versions = map[string]uint64{}

func getKey(nodeID, shardID uint64) string {
	return fmt.Sprintf("%d_%d", nodeID, shardID)
}

func GetSnapshotVersions() map[string]uint64 {
	return Versions
}

func GetSnapshotVersion(nodeID, shardID uint64) uint64 {
	key := getKey(nodeID, shardID)
	if v, ok := Versions[key]; ok {
		return v
	}
	return 1 // 預設為 snapshot version 1
}

func SetSnapshotVersion(nodeID, shardID, version uint64) uint64 {
	key := getKey(nodeID, shardID)
	Versions[key] = version
	return version
}
