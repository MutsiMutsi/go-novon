package core

// Define a type for event handlers
type EventHandler func(data interface{})

// Event struct holds a list of listeners
type Event struct {
	listeners []EventHandler
}

// Register a new listener
func (e *Event) Subscribe(handler EventHandler) {
	e.listeners = append(e.listeners, handler)
}

// Raise the event (notify all listeners)
func (e *Event) Emit(data interface{}) {
	for _, handler := range e.listeners {
		handler(data)
	}
}
