package core

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/nknorg/nkn-sdk-go"
	"github.com/nknorg/nkngomobile"
)

var viewerAddresses []string
var viewerSubClientAddresses [VIEWER_SUB_CLIENTS]*nkngomobile.StringArray

// Viewers is a thread-safe collection of message addresses with last receive timestamps.
type Viewers struct {
	messages      map[string]*messageData
	viewerQuality map[string]int
	mutex         sync.RWMutex
	timeout       time.Duration
}

// messageData holds the last received time for an address.
type messageData struct {
	lastTime time.Time
}

// NewViewers creates a new Viewers with a specified timeout duration.
func NewViewers(timeout time.Duration) *Viewers {
	return &Viewers{
		messages:      make(map[string]*messageData),
		viewerQuality: make(map[string]int),
		mutex:         sync.RWMutex{},
		timeout:       timeout,
	}
}

// AddOrUpdateAddress updates the last received time for an address or adds it if not present.
func (ms *Viewers) AddOrUpdateAddress(address string) (isNew bool) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	data, ok := ms.messages[address]
	if !ok {
		data = &messageData{lastTime: time.Now()}
		ms.messages[address] = data
		ms.viewerQuality[address] = 1
		ms.SetAddresses()
	} else {
		data.lastTime = time.Now()
	}

	return !ok
}

// GetAddresses returns an array of all addresses in the store.
func (ms *Viewers) SetAddresses() {
	//addresses strings
	addresses := make([]string, 0, len(ms.messages))
	for address := range ms.messages {
		addresses = append(addresses, address)
	}
	viewerAddresses = addresses

	//create nkn string arrays for all viewer subclients
	nknAddrStrings := [VIEWER_SUB_CLIENTS]*nkngomobile.StringArray{}
	for i := 0; i < VIEWER_SUB_CLIENTS; i++ {
		prefixedAddresses := make([]string, len(viewerAddresses))
		for j, address := range viewerAddresses {
			prefixedAddresses[j] = "__" + strconv.Itoa(i) + "__." + address
		}

		nknAddrStrings[i] = nkn.NewStringArray(prefixedAddresses...)
	}

	viewerSubClientAddresses = nknAddrStrings
}

// Cleanup removes addresses from the store that haven't received messages in the timeout duration.
func (ms *Viewers) Cleanup() {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	anyDeleted := false

	timeout := time.Now().Add(-ms.timeout)
	for address, data := range ms.messages {
		if data.lastTime.Before(timeout) {
			delete(ms.messages, address)
			log.Println("viewer left - timeout")
			anyDeleted = true
		}
	}

	if anyDeleted {
		ms.SetAddresses()
	}
}

func (ms *Viewers) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("viewerCleanup: stopping")
				return
			default:
				time.Sleep(interval)
				ms.Cleanup()
			}
		}
	}()
}

// Remove an address
func (ms *Viewers) Remove(address string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	delete(ms.messages, address)
	log.Println("viewer left - disconnected")
	ms.SetAddresses()
}
