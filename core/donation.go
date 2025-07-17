package core

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/MutsiMutsi/go-novon/core/json"
	"github.com/nknorg/nkn-sdk-go"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/pb"
	"google.golang.org/protobuf/proto"
)

var donationRegex = regexp.MustCompile(`donate[0-9]+`)
var donationMap map[string]string = make(map[string]string)

func ValidateDonation(streamer *Streamer, message *ChatMessage, allowMempool bool) (err error) {

	//Check if message text contains donation, if so validate, otherwise early true nothing to check.
	var donationSum int
	donateMatches := donationRegex.FindAllString(message.Text, -1)

	//No donations always valid
	if len(donateMatches) == 0 {
		return nil
	}

	for _, match := range donateMatches {
		amount, err := strconv.Atoi(match[6:]) // Remove "donate" from the start
		if err != nil {
			fmt.Println("Error parsing amount:", err)
			continue // Skip to the next match if there's an error
		}
		donationSum += amount
	}

	//probably donation0 was included in the message, this is not an actual donation
	if donationSum == 0 {
		return nil
	}

	//donation but no hash always invalid
	if donationSum > 0 && len(message.Hash) != 64 {
		return errors.New("no tx hash")
	}

	//get the src wallet address
	srcPk, _ := nkn.ClientAddrToPubKey(message.Src)
	srcAddr, _ := nkn.PubKeyToWalletAddr(srcPk)

	transaction := &json.Transaction{}
	if allowMempool {
		transaction, err = getTransactionFromMempool(message.Hash, srcAddr)
		if err != nil {
			return err
		}
	}

	if transaction != nil && transaction.Hash == "" {
		err = getTransactionWithRetry(context.Background(), message.Hash, transaction)
		if err != nil {
			return err
		}
	}

	//donation is has to be known
	hash, exists := donationMap[transaction.Attributes]
	if !exists {
		return errors.New("this donation id does not exist")
	}

	if len(hash) > 0 {
		return errors.New("this donation was already received")
	}

	donationMap[transaction.Attributes] = transaction.Hash

	//incorrect txtype always invalid
	if transaction.TxType != "TRANSFER_ASSET_TYPE" {
		return errors.New("incorrect txtype")
	}

	payloadBytes, err := hex.DecodeString(transaction.PayloadData)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(payloadBytes)

	transferAsset := new(pb.TransferAsset)
	err = proto.Unmarshal(payloadBytes, transferAsset)
	if err != nil {
		fmt.Println(err)
	}

	//verify donation amount with transfer amount
	if int64(donationSum)*int64(nkn.AmountUnit) != transferAsset.Amount {
		return errors.New("transfer amount mismatch")
	}

	programHashSender, _ := common.Uint160ParseFromBytes(transferAsset.Sender)
	programHashRecipient, _ := common.Uint160ParseFromBytes(transferAsset.Recipient)
	senderAddr, _ := programHashSender.ToAddress()
	recipientAddr, _ := programHashRecipient.ToAddress()

	//validate transfer sender is the message sender
	if senderAddr != srcAddr {
		return errors.New("transfer sender is not message src")
	}

	//validate recipient is this stream host
	if recipientAddr != streamer.nknClient.Account().WalletAddress() {
		return errors.New("transfer recipient is not host address")
	}

	return nil
}

func getTransactionWithRetry(ctx context.Context, hash string, transaction *json.Transaction) (err error) {
	for i := 0; i < 10; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err = nkn.RPCCall(timeoutCtx, "gettransaction", map[string]interface{}{"hash": hash}, transaction, nkn.GetDefaultRPCConfig())
		if err == nil {
			return nil // Success!
		}

		fmt.Printf("Transaction not found yet, retrying in %v...\n", 5*time.Second)
		time.Sleep(5 * time.Second)
	}

	return errors.New("transaction not found in time")
}

func getTransactionFromMempool(hash string, sender string) (*json.Transaction, error) {
	var transactions []json.Transaction
	requestBody := map[string]interface{}{"action": "txnlist", "address": sender}

	for i := 0; i < 5; i++ {
		err := nkn.RPCCall(context.Background(), "getrawmempool", requestBody, &transactions, nkn.GetDefaultRPCConfig())
		if err != nil {
			return nil, err
		}

		for _, tx := range transactions {
			if tx.Hash == hash {
				return &tx, nil
			}
		}

		fmt.Printf("Transaction not in mempool, retrying in %v...\n", time.Second)
		time.Sleep(time.Second)
	}

	return nil, nil
}

func generateDonationEntry() string {
	rngBytes, _ := nkn.RandomBytes(32)
	hex := hex.EncodeToString(rngBytes)
	donationMap[hex] = ""
	return hex
}
