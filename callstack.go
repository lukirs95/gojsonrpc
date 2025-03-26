package wsjsonrpc

import (
	"fmt"
)

func newCallStack() *callStack {
	return &callStack{make(map[Id]*Response)}
}

func (stack *callStack) push(id Id, responseChannel *Response) {
	stack.callStack[id] = responseChannel
}

func (stack *callStack) pop(id Id) (*Response, error) {
	if responseChannel, ok := stack.callStack[id]; ok {
		delete(stack.callStack, id)
		return responseChannel, nil
	} else {
		return nil, fmt.Errorf("call with id %d is not in callStack", id)
	}
}

func (stack *callStack) empty() bool {
	if len(stack.callStack) == 0 {
		return true
	} else {
		return false
	}
}
