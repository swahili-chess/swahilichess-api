package nextsms

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const next_url = "https://messaging-service.co.tz/api/sms/v1/text/single"

const sourceAddr = "Chess"

type NextSmS struct {
	Username string
	Password string
}

type Message struct {
	To        string `json:"to"`
	Status    Status `json:"status"`
	MessageID int64  `json:"messageId"`
	SMSCount  int    `json:"smsCount"`
	Message   string `json:"message"`
}

type Status struct {
	GroupID     int    `json:"groupId"`
	GroupName   string `json:"groupName"`
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Define a struct to encompass the entire JSON body
type Payload struct {
	Messages []Message `json:"messages"`
}

func New(username string, password string) NextSmS {
	return NextSmS{
		Username: username,
		Password: password,
	}
}

func (n NextSmS) SendSmS(msg string, recipient_phone string) error {

	payload := struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Text      string `json:"text"`
		Reference string `json:"reference"`
	}{
		From:      sourceAddr,
		To:        recipient_phone[1:], // exclude the +
		Text:      msg,
		Reference: uuid.NewString(),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed encoding JSON payload", "error", err)
		return err
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", n.Username, n.Password)))
	req, err := http.NewRequest("POST", next_url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		slog.Error("failed creating HTTP request to send to nextsms", "error", err)
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", encoded))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		slog.Error("failed sending request to nextsms", "error", err)
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.New("nextsms failed to send sms")
	}

	var p Payload

	err = json.NewDecoder(res.Body).Decode(&p)
	if err != nil {
		slog.Error(fmt.Sprintf("Error decoding JSON response: %s", err))
		return err
	}

	slog.LogAttrs(context.Background(),
		slog.LevelInfo,
		"Success response from NextSmS",
		slog.Int("http-status", res.StatusCode),
		slog.String("body", fmt.Sprintf("%+v", p)),
	)

	return nil
}
