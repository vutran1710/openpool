package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Bot struct {
	token      string
	httpClient *http.Client
}

func NewBot(token string) *Bot {
	return &Bot{
		token:      token,
		httpClient: &http.Client{},
	}
}

func (b *Bot) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", b.token, method)
}

func (b *Bot) CreateGroup(title string) (int64, error) {
	payload := map[string]any{
		"title": title,
	}
	body, _ := json.Marshal(payload)

	resp, err := b.httpClient.Post(
		b.apiURL("createChat"),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return 0, fmt.Errorf("creating group: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			ID int64 `json:"id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Result.ID, nil
}

func (b *Bot) SendMessage(chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	body, _ := json.Marshal(payload)

	resp, err := b.httpClient.Post(
		b.apiURL("sendMessage"),
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (b *Bot) GetUpdates(offset int64) ([]Update, error) {
	url := fmt.Sprintf("%s?offset=%d&timeout=30", b.apiURL("getUpdates"), offset)
	resp, err := b.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("getting updates: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Result, nil
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	Date      int64  `json:"date"`
}

type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type"`
}
