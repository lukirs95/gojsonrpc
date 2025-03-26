package wsjsonrpc

import (
	"errors"
)

var (
	ErrDuplicateId = errors.New("request id already in map")
	ErrIdNotFound  = errors.New("request id not found")
)

type requestResponseMap map[RequestId]ResponseChan

func (m requestResponseMap) push(id RequestId, responseChan ResponseChan) error {
	if _, ok := m[id]; ok {
		return ErrDuplicateId
	}
	m[id] = responseChan
	return nil
}

func (m requestResponseMap) pop(id RequestId) (ResponseChan, error) {
	if r, ok := m[id]; ok {
		delete(m, id)
		return r, nil
	} else {
		return nil, ErrIdNotFound
	}

}

func (m requestResponseMap) empty() bool {
	return len(m) == 0
}
