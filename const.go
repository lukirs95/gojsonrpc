package wsjsonrpc

const VERSION = "2.0"

const (
	M_TYPE_REQUEST MessageType = iota + 1
	M_TYPE_NOTIFY
	M_TYPE_RESPONSE
)

const (
	R_TYPE_RESULT ResponseType = iota + 1
	R_TYPE_ERROR
	R_TYPE_DELETED
)
