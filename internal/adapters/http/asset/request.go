package asset

type RequestAdd struct {
	UID      string  `json:"uid" binding:"required"`
	Currency string  `json:"currency" binding:"required"`
	Amount   float64 `json:"amount" binding:"required"`
}
