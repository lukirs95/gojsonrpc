package wsjsonrpc

import "fmt"

func newSubscriberRegistry() *subscriberRegistry {
	return &subscriberRegistry{make(map[Method]*Subscriber)}
}

func (subscriberRegistry *subscriberRegistry) push(method Method, subscriber *Subscriber) {
	subscriberRegistry.subscriber[method] = subscriber
}

func (subscriberRegistry *subscriberRegistry) pop(method Method) (*Subscriber, error) {
	if channel, ok := subscriberRegistry.subscriber[method]; ok {
		delete(subscriberRegistry.subscriber, method)
		return channel, nil
	} else {
		return nil, fmt.Errorf("subscriber for method %s is not in registry", method)
	}
}

func (subscriberRegistry *subscriberRegistry) empty() bool {
	if len(subscriberRegistry.subscriber) == 0 {
		return true
	} else {
		return false
	}
}
