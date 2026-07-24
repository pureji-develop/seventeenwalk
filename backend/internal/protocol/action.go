package protocol

// Action payload structs (Client → Server).

type ReadyPayload struct {
	Ready bool `json:"ready"`
}

type SelectDoraPayload struct {
	TileID int `json:"tileId"`
}

type SubmitHandPayload struct {
	TileIDs []int `json:"tileIds"`
}

type DiscardTilePayload struct {
	TileID int `json:"tileId"`
}

type SetOptionsPayload struct {
	// TimeoutPreset: "120" | "180" | "300" | "inf"（推奨）
	TimeoutPreset string `json:"timeoutPreset"`
	// BuildTimeoutSec: 互換用。0 または TimeoutPreset=inf で無限
	BuildTimeoutSec int `json:"buildTimeoutSec"`
	// TotalRounds: 4 | 6 | 8 | 10（省略時は変更しない／サーバ既定）
	TotalRounds int  `json:"totalRounds"`
	WallSorted  bool `json:"wallSorted"`
}
