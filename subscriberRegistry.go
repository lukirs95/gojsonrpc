package gojsonrpc

import (
	"context"
	"sync"
)

type subscriber struct {
	notifications chan<- Notification
	ctx           context.Context
}

// subscriberRegistry maps a method to the subscriber of its notifications.
type subscriberRegistry struct {
	subscribers map[Method]subscriber
	sync.RWMutex
}

func (r *subscriberRegistry) push(method Method, s subscriber) {
	r.Lock()
	defer r.Unlock()
	r.subscribers[method] = s
}

func (r *subscriberRegistry) get(method Method) (subscriber, bool) {
	r.RLock()
	defer r.RUnlock()
	s, ok := r.subscribers[method]
	return s, ok
}

func (r *subscriberRegistry) pop(method Method) bool {
	r.Lock()
	defer r.Unlock()
	_, ok := r.subscribers[method]
	delete(r.subscribers, method)
	return ok
}
