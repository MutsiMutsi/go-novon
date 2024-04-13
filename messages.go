package main

import (
	"encoding/json"
	"fmt"

	"github.com/nknorg/nkn-sdk-go"
)

var chatId uint64 = 0

// Message struct represents a generic message with a type and content
type Message struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

type ChatMessage struct {
	Id   uint64 `json:"id,string"`
	Text string `json:"text"`
	Hash string `json:"hash"`
	Src  string `json:"src"`
	Role string `json:"role"`
}

func DecodeMessage(receivedMessage *nkn.Message) {
	// Unmarshal the JSON into a Message struct
	var msg Message
	if err := json.Unmarshal(receivedMessage.Data, &msg); err != nil {
		fmt.Println("Error deserializing JSON:", err)
		return
	}

	// Handle the message based on its type
	switch msg.Type {
	case "chat-message":
		chatMsg := &ChatMessage{
			Src: receivedMessage.Src,
		}
		if err := json.Unmarshal(msg.Content, &chatMsg); err != nil {
			fmt.Println("Error unmarshalling message content:", err)
			return
		}
		HandleChatMessage(chatMsg, receivedMessage)
	default:
		fmt.Println("Unknown message type:", msg.Type)
	}
}

func HandleChatMessage(msg *ChatMessage, nknMessage *nkn.Message) {
	go func() {
		fmt.Println("Message:", msg.Text)

		err := ValidateDonation(msg, true)
		if err != nil {
			fmt.Println("donation validation error", err.Error())
			nknMessage.Reply([]byte("error: donation validation error - " + err.Error()))
			return
		} else {
			nknMessage.Reply([]byte("success"))
		}

		msg.Id = chatId
		if msg.Src == config.Owner {
			msg.Role = "owner"
		}
		chatId++

		binary, err := json.Marshal(msg)
		if err != nil {
			panic(err)
		}
		publish(binary)
	}()
}
