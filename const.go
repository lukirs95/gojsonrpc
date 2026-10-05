package gojsonrpc

const jsonrpcVersion = "2.0"

type messageType int

const (
	messageTypeRequest messageType = iota + 1
	messageTypeNotification
	messageTypeResponse
)
