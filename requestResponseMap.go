package gojsonrpc

import (
	"errors"
	"sync"
)

var (
	ErrDuplicateId = errors.New("request id already in map")
	ErrIdNotFound  = errors.New("request id not found")
)

// requestResponseMap maps a RequestId to a response channel
type requestResponseMap struct {
	store map[RequestId]ResponseChan
	sync.RWMutex
}

// push adds a new ResponseId to ResponseChannel Mapping and returns ErrDuplicateId
// if id is already in map
func (m *requestResponseMap) push(id RequestId, responseChan ResponseChan) error {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.store[id]; ok {
		return ErrDuplicateId
	}
	m.store[id] = responseChan
	return nil
}

func (m *requestResponseMap) pop(id RequestId) (ResponseChan, error) {
	m.Lock()
	defer m.Unlock()
	if r, ok := m.store[id]; ok {
		delete(m.store, id)
		return r, nil
	} else {
		return nil, ErrIdNotFound
	}

}

func (m *requestResponseMap) empty() bool {
	m.RLock()
	defer m.RUnlock()
	return len(m.store) == 0
}
