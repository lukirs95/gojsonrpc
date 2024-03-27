package jsonrpc

import (
	"encoding/json"
	"fmt"
)

// type UnknownMessage struct {
// 	messageType  MessageType
// 	Request      RpcRequest
// 	Notification RpcNotification
// 	Response     RpcRawResponse
// }

type helperMessage struct {
	Version Version         `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Method  json.RawMessage `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func (m *UnknownMessage) UnmarshalJSON(raw []byte) error {
	var helper helperMessage
	err := json.Unmarshal(raw, &helper)
	if err != nil {
		return err
	}
	if helper.Error != nil || helper.Result != nil {
		var response RpcRawResponse
		err = json.Unmarshal(raw, &response)
		if err != nil {
			return err
		}
		m.messageType = M_TYPE_RESPONSE
		m.Response = response
		return nil
	}
	if helper.Id == nil {
		var notification RpcNotification
		err = json.Unmarshal(raw, &notification)
		if err != nil {
			return err
		}
		m.messageType = M_TYPE_NOTIFY
		m.Notification = notification
		return nil
	}
	if helper.Method != nil {
		var request RpcRequest
		err = json.Unmarshal(raw, &request)
		if err != nil {
			return err
		}
		m.messageType = M_TYPE_REQUEST
		m.Request = request
		return nil
	}
	return fmt.Errorf("received unknown rpc message")
}

func (m *UnknownMessage) IsRequest() bool {
	return m.messageType == M_TYPE_REQUEST
}
