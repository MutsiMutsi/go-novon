package main

import (
	"encoding/json"
	"fmt"

	"github.com/nknorg/nkn-sdk-go"
)

// Message struct represents a generic message with a type and content
type Message struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

type ChatMessage struct {
	Text string `json:"text"`
	Src  string `json:"src"`
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
		chatMsg := ChatMessage{
			Src: receivedMessage.Src,
		}
		if err := json.Unmarshal(msg.Content, &chatMsg); err != nil {
			fmt.Println("Error unmarshalling message content:", err)
			return
		}
		HandleChatMessage(chatMsg, receivedMessage.Src)
	default:
		fmt.Println("Unknown message type:", msg.Type)
	}
}

func HandleChatMessage(msg ChatMessage, src string) {
	fmt.Println("Message:", msg.Text)

	binary, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	_, err = client.Send(nkn.NewStringArray(viewerAddresses...), binary, segmentSendConfig)
	if err != nil {
		panic(err)
	}
}
