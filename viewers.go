package main

import (
	"log"
	"sync"
	"time"
)

// Viewers is a thread-safe collection of message addresses with last receive timestamps.
type Viewers struct {
	messages map[string]*messageData
	mutex    sync.RWMutex
	timeout  time.Duration
}

// messageData holds the last received time for an address.
type messageData struct {
	lastTime time.Time
}

// NewViewers creates a new Viewers with a specified timeout duration.
func NewViewers(timeout time.Duration) *Viewers {
	return &Viewers{
		messages: make(map[string]*messageData),
		mutex:    sync.RWMutex{},
		timeout:  timeout,
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
	} else {
		data.lastTime = time.Now()
	}

	return !ok
}

// GetAddresses returns an array of all addresses in the store.
func (ms *Viewers) GetAddresses() []string {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	addresses := make([]string, 0, len(ms.messages))
	for address := range ms.messages {
		addresses = append(addresses, address)
	}
	return addresses
}

// Cleanup removes addresses from the store that haven't received messages in the timeout duration.
func (ms *Viewers) Cleanup() {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	timeout := time.Now().Add(-ms.timeout)
	for address, data := range ms.messages {
		if data.lastTime.Before(timeout) {
			delete(ms.messages, address)
			log.Println("viewer left - timeout")
		}
	}
}

func (ms *Viewers) StartCleanup(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			ms.Cleanup()
		}
	}()
}

// Remove an address
func (ms *Viewers) Remove(address string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	delete(ms.messages, address)
	log.Println("viewer left - disconnected")
}
