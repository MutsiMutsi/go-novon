package core

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

type DeleteChatMessage struct {
	MsgId uint64 `json:"msgId,string"`
}

func (s *Streamer) DecodeMessage(receivedMessage *nkn.Message) {
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
		s.HandleChatMessage(chatMsg, receivedMessage)
	case "delete-chat-message":
		{
			if receivedMessage.Src == s.config.Owner {
				var deleteMsg DeleteChatMessage
				if err := json.Unmarshal(msg.Content, &deleteMsg); err != nil {
					fmt.Println("Error unmarshalling message content:", err)
					return
				}

				s.publishText(string(receivedMessage.Data))
			}
		}
	default:
		fmt.Println("Unknown message type:", msg.Type, "content:", string(msg.Content))
	}
}

func (s *Streamer) HandleChatMessage(msg *ChatMessage, nknMessage *nkn.Message) {
	go func() {
		fmt.Println("Message:", msg.Text)

		err := ValidateDonation(s, msg, true)
		if err != nil {
			fmt.Println("donation validation error", err.Error())
			nknMessage.Reply([]byte("error: donation validation error - " + err.Error()))
			return
		} else {
			nknMessage.Reply([]byte("success"))
		}

		msg.Id = chatId
		if msg.Src == s.config.Owner {
			msg.Role = "owner"
		}
		chatId++

		binary, err := json.Marshal(msg)
		if err != nil {
			panic(err)
		}
		s.publish(binary)
	}()
}
