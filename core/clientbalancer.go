package core

import (
	"fmt"
	"log"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/nknorg/nkn-sdk-go"
	"github.com/nknorg/nkn-sdk-go/payloads"
	"github.com/nknorg/nkngomobile"
)

var clientSendIndex = 0

func (s *Streamer) getNextClient() *nkn.Client {
	clientId := clientSendIndex % NUM_SUB_CLIENTS
	client := s.nknClient.GetClient(clientId)
	clientSendIndex++

	if client == nil {
		client = s.getNextClient()
	}

	return client
}

func (s *Streamer) publish(data []byte) {
	//Foreach chunk generate a message id and predefine the payload to reuse
	msgId, _ := nkn.RandomBytes(nkn.MessageIDSize)
	msgPayload := &payloads.Payload{
		Type:      payloads.PayloadType_BINARY,
		NoReply:   true,
		MessageId: msgId,
		Data:      data,
	}

	//Send VIEWER_SUB_CLIENTS times everytime with the next subclient in queue
	for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
		go s.getNextClient().SendPayload(viewerSubClientAddresses[i], msgPayload, segmentSendConfig)
	}
}

func (s *Streamer) publishQualityLevels(qualityData ...[][]byte) {
	qualityLevels := len(qualityData)
	qualityAddrStrings := make([][]string, qualityLevels)
	qualityNknAddrStrings := make([][]*nkngomobile.StringArray, qualityLevels)

	// Build address slices
	for i := range qualityData {
		qualityNknAddrStrings[i] = make([]*nkngomobile.StringArray, VIEWER_SUB_CLIENTS)
	}

	// Build viewer lists for each quality
	for k := range s.viewers.messages {
		qualityLevel := min(s.viewers.viewerQuality[k], qualityLevels)
		qualityAddrStrings[qualityLevel] = append(qualityAddrStrings[qualityLevel], k)
	}

	// Convert to multiclient recipient nkn addreses
	for q := 0; q < qualityLevels; q++ {
		// Preprocess addresses for this quality level (similar to original code)
		for j := 0; j < VIEWER_SUB_CLIENTS; j++ {
			prefixedAddresses := make([]string, len(qualityAddrStrings[q]))
			for k, viewer := range qualityAddrStrings[q] {
				prefixedAddresses[k] = "__" + strconv.Itoa(j) + "__." + viewer
			}
			qualityNknAddrStrings[q][j] = nkn.NewStringArray(prefixedAddresses...)
		}
	}

	// Send the chunks to each quality level
	for q := 0; q < qualityLevels; q++ {
		for _, v := range qualityData[q] {
			msgId, _ := nkn.RandomBytes(nkn.MessageIDSize)
			msgPayload := &payloads.Payload{
				Type:      payloads.PayloadType_BINARY,
				NoReply:   true,
				MessageId: msgId,
				Data:      v,
			}

			for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
				go s.getNextClient().SendPayload(qualityNknAddrStrings[q][i], msgPayload, segmentSendConfig)
			}
		}
	}
}

func (s *Streamer) publishText(text string) {
	//Foreach chunk generate a message id and predefine the payload to reuse
	msgId, _ := nkn.RandomBytes(nkn.MessageIDSize)

	data, err := proto.Marshal(&payloads.TextData{Text: text})
	if err != nil {
		fmt.Println(err.Error())
	}

	msgPayload := &payloads.Payload{
		Type:      payloads.PayloadType_TEXT,
		NoReply:   true,
		MessageId: msgId,
		Data:      data,
	}

	//Send VIEWER_SUB_CLIENTS times everytime with the next subclient in queue
	for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
		go s.getNextClient().SendPayload(viewerSubClientAddresses[i], msgPayload, segmentSendConfig)
	}
}

func (s *Streamer) sendToClient(address string, data []byte) {
	msgId, _ := nkn.RandomBytes(nkn.MessageIDSize)
	msgPayload := &payloads.Payload{
		Type:      payloads.PayloadType_BINARY,
		NoReply:   true,
		MessageId: msgId,
		Data:      data,
	}

	for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
		go s.getNextClient().SendPayload(nkn.NewStringArray("__"+strconv.Itoa(i)+"__."+address), msgPayload, &nkn.MessageConfig{
			Unencrypted:       true,
			NoReply:           true,
			MaxHoldingSeconds: 0,
		})
	}
}

func (s *Streamer) reply(data []byte, msg *nkn.Message) {
	payload, err := nkn.NewReplyPayload(data, msg.MessageID)
	if err != nil {
		log.Println(err)
	}

	for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
		go s.getNextClient().SendPayload(nkn.NewStringArray("__"+strconv.Itoa(i)+"__."+msg.Src), payload, &nkn.MessageConfig{
			Unencrypted:       true,
			NoReply:           true,
			MaxHoldingSeconds: 0,
		})
	}
}

func (s *Streamer) replyText(text string, msg *nkn.Message) {
	payload, err := nkn.NewReplyPayload(text, msg.MessageID)
	if err != nil {
		log.Println(err)
	}

	for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
		go s.getNextClient().SendPayload(nkn.NewStringArray("__"+strconv.Itoa(i)+"__."+msg.Src), payload, &nkn.MessageConfig{
			Unencrypted:       true,
			NoReply:           true,
			MaxHoldingSeconds: 0,
		})
	}
}
