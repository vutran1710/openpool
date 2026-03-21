package limits

const (
	MaxChatMessage    = 4096   // 4 KB — ~500 words plaintext
	MaxRelayFrame     = 8192   // 8 KB — encrypted chat + overhead
	MaxMessageContent = 65536  // 64 KB — issue body / comment content
	MaxBinFile        = 262144 // 256 KB — encrypted profile .bin
)
