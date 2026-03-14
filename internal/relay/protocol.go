package relay

type InboundFrame struct {
	Type string `json:"type"`

	UserHash  string `json:"user_hash,omitempty"`
	PoolRepo  string `json:"pool_repo,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`

	To   string `json:"to,omitempty"`
	Body string `json:"body,omitempty"`
}

type OutboundMessage struct {
	Type string `json:"type"`
	From string `json:"from,omitempty"`
	Body string `json:"body,omitempty"`
	Ts   int64  `json:"ts,omitempty"`

	Nonce string `json:"nonce,omitempty"`
	Error string `json:"error,omitempty"`
}
