package snapshot

type RequestSetSnapshot struct {
	Version string `json:"version" binding:"required"`
}
