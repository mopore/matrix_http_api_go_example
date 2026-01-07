package matrixapi

type WhoamiResponse struct {
	UserID string `json:"user_id"`
}

type MatrixEvent struct {
	EventID string `json:"event_id,omitempty"`
	Type    string `json:"type,omitempty"`
	Sender  string `json:"sender,omitempty"`
	Content struct {
		MsgType string `json:"msgtype,omitempty"`
		Body    string `json:"body,omitempty"`
	} `json:"content"`
}

type SyncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Invite map[string]any `json:"invite"`
		Join   map[string]struct {
			Timeline struct {
				Events []MatrixEvent `json:"events"`
			} `json:"timeline"`
		} `json:"join"`
	} `json:"rooms"`
}
