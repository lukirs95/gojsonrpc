package gojsonrpc

import "sync"

// requestResponseMap maps the id of a pending request to the channel its
// response is sent on.
type requestResponseMap struct {
	store map[requestID]chan response
	sync.Mutex
}

func (m *requestResponseMap) push(id requestID, responseChan chan response) {
	m.Lock()
	defer m.Unlock()
	m.store[id] = responseChan
}

// pop removes the request and returns its channel. Only the caller that pops
// an entry sends on the channel.
func (m *requestResponseMap) pop(id requestID) (chan response, bool) {
	m.Lock()
	defer m.Unlock()
	responseChan, ok := m.store[id]
	delete(m.store, id)
	return responseChan, ok
}

// popAll removes all pending requests and returns their response channels.
func (m *requestResponseMap) popAll() []chan response {
	m.Lock()
	defer m.Unlock()
	channels := make([]chan response, 0, len(m.store))
	for id, responseChan := range m.store {
		channels = append(channels, responseChan)
		delete(m.store, id)
	}
	return channels
}
