package limits

import "testing"

func TestConstants(t *testing.T) {
	if MaxChatMessage != 4096 {
		t.Errorf("MaxChatMessage = %d, want 4096", MaxChatMessage)
	}
	if MaxRelayFrame != 8192 {
		t.Errorf("MaxRelayFrame = %d, want 8192", MaxRelayFrame)
	}
	if MaxMessageContent != 65536 {
		t.Errorf("MaxMessageContent = %d, want 65536", MaxMessageContent)
	}
	if MaxBinFile != 262144 {
		t.Errorf("MaxBinFile = %d, want 262144", MaxBinFile)
	}
}
