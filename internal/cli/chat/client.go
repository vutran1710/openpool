package chat

import (
	"crypto/ed25519"
	"fmt"
	"log"

	relayclient "github.com/vutran1710/dating-dev/internal/cli/relay"
	"github.com/vutran1710/dating-dev/internal/limits"
)

type ChatClient struct {
	Relay *relayclient.Client
	DB    *ConversationDB
	OnMsg func(peerMatchHash string) // notify UI of new message
}

func NewChatClient(relay *relayclient.Client, db *ConversationDB) *ChatClient {
	c := &ChatClient{Relay: relay, DB: db}
	relay.OnMessage(func(senderMatchHash string, plaintext []byte) {
		c.handleIncoming(senderMatchHash, plaintext)
	})
	return c
}

func (c *ChatClient) handleIncoming(senderMatchHash string, plaintext []byte) {
	if len(plaintext) > limits.MaxChatMessage {
		log.Printf("dropping oversized message from %s: %d bytes", senderMatchHash, len(plaintext))
		return
	}
	if err := c.DB.SaveMessage(senderMatchHash, string(plaintext), false); err != nil {
		log.Printf("save incoming: %v", err)
	}
	if c.OnMsg != nil {
		c.OnMsg(senderMatchHash)
	}
}

func (c *ChatClient) Send(peerMatchHash, text string) error {
	if len(text) > limits.MaxChatMessage {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(text), limits.MaxChatMessage)
	}
	if err := c.Relay.SendMessage(peerMatchHash, text); err != nil {
		return err
	}
	return c.DB.SaveMessage(peerMatchHash, text, true)
}

func (c *ChatClient) History(peerMatchHash string) ([]Message, error) {
	return c.DB.LoadHistory(peerMatchHash)
}

func (c *ChatClient) Conversations() ([]Conversation, error) {
	return c.DB.ListConversations()
}

func (c *ChatClient) MarkRead(peerMatchHash string) error {
	return c.DB.MarkRead(peerMatchHash)
}

func (c *ChatClient) PersistGreeting(peerMatchHash, greeting string) error {
	return c.DB.PersistGreeting(peerMatchHash, greeting)
}

func (c *ChatClient) UnreadTotal() (int, error) {
	return c.DB.UnreadTotal()
}

func (c *ChatClient) SetPeerKey(peerMatchHash string, peerPub ed25519.PublicKey) {
	c.Relay.SetPeerKey(peerMatchHash, peerPub)
	c.DB.SavePeerKey(peerMatchHash, []byte(peerPub))
}

func (c *ChatClient) GetPeerKey(peerMatchHash string) (ed25519.PublicKey, error) {
	pubkey, err := c.DB.GetPeerKey(peerMatchHash)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(pubkey), nil
}

// LoadPeerKeys loads all stored peer keys into the relay client.
func (c *ChatClient) LoadPeerKeys() {
	convos, err := c.DB.ListConversations()
	if err != nil {
		return
	}
	for _, conv := range convos {
		pub, err := c.DB.GetPeerKey(conv.PeerMatchHash)
		if err == nil && len(pub) == ed25519.PublicKeySize {
			c.Relay.SetPeerKey(conv.PeerMatchHash, ed25519.PublicKey(pub))
		}
	}
}
