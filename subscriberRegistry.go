package gojsonrpc

import "fmt"

func newSubscriberRegistry() *subscriberRegistry {
	return &subscriberRegistry{subscriber: make(map[Method]*Subscriber)}
}

func (subscriberRegistry *subscriberRegistry) push(method Method, subscriber *Subscriber) {
	subscriberRegistry.Lock()
	defer subscriberRegistry.Unlock()
	subscriberRegistry.subscriber[method] = subscriber
}

func (subscriberRegistry *subscriberRegistry) get(method Method) (*Subscriber, bool) {
	subscriberRegistry.RLock()
	defer subscriberRegistry.RUnlock()
	subscriber, ok := subscriberRegistry.subscriber[method]
	return subscriber, ok
}

func (subscriberRegistry *subscriberRegistry) pop(method Method) (*Subscriber, error) {
	subscriberRegistry.Lock()
	defer subscriberRegistry.Unlock()
	if channel, ok := subscriberRegistry.subscriber[method]; ok {
		delete(subscriberRegistry.subscriber, method)
		return channel, nil
	} else {
		return nil, fmt.Errorf("subscriber for method %s is not in registry", method)
	}
}
