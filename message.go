package gojsonrpc

import (
	"encoding/json"
	"fmt"
)

// incomingMessage is any JSON-RPC message received from the server. Only the
// field matching messageType is set.
type incomingMessage struct {
	messageType  messageType
	request      rpcRequest
	notification rpcNotification
	response     rpcResponse
}

type helperMessage struct {
	Id     json.RawMessage `json:"id"`
	Method json.RawMessage `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (m *incomingMessage) UnmarshalJSON(raw []byte) error {
	var helper helperMessage
	if err := json.Unmarshal(raw, &helper); err != nil {
		return err
	}
	switch {
	case helper.Error != nil || helper.Result != nil:
		m.messageType = messageTypeResponse
		return json.Unmarshal(raw, &m.response)
	case helper.Id == nil:
		m.messageType = messageTypeNotification
		return json.Unmarshal(raw, &m.notification)
	case helper.Method != nil:
		m.messageType = messageTypeRequest
		return json.Unmarshal(raw, &m.request)
	}
	return fmt.Errorf("received unknown rpc message")
}
