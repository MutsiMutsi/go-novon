package main

import (
	"log"
	"strconv"

	"github.com/nknorg/nkn-sdk-go"
	"github.com/nknorg/nkn-sdk-go/payloads"
)

var clientSendIndex = 0

func publish(data []byte) {
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
		clientId := clientSendIndex % NUM_SUB_CLIENTS
		go client.GetClient(clientId).SendPayload(viewerSubClientAddresses[i], msgPayload, segmentSendConfig)
		clientSendIndex++
	}
}

func reply(data []byte, msg *nkn.Message) {
	payload, err := nkn.NewReplyPayload(data, msg.MessageID)
	if err != nil {
		log.Println(err)
	}

	for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
		clientId := clientSendIndex % NUM_SUB_CLIENTS
		go client.GetClient(clientId).SendPayload(nkn.NewStringArray("__"+strconv.Itoa(i)+"__."+msg.Src), payload, &nkn.MessageConfig{
			Unencrypted:       true,
			NoReply:           true,
			MaxHoldingSeconds: 0,
		})
		clientId++
	}
}
