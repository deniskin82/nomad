// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
)

const (
	ACLCheckNodeRead   = "node-read"
	ACLCheckManagement = "management"
	aclCacheSize       = 32
)

type EventBrokerCfg struct {
	EventBufferSize int64
	Logger          hclog.Logger
}

type EventBroker struct {
	// mu protects subscriptions
	mu            sync.Mutex
	subscriptions *subscriptions

	// eventBuf stores a configurable amount of events in memory
	eventBuf *eventBuffer

	// publishCh is used to send messages from an active txn to a goroutine which
	// publishes events, so that publishing can happen asynchronously from
	// the Commit call in the FSM hot path.
	publishCh chan *structs.Events

	aclCh chan structs.Event

	logger hclog.Logger
}

// NewEventBroker returns an EventBroker for publishing change events.
// A goroutine is run in the background to publish events to an event buffer.
// Cancelling the context will shutdown the goroutine to free resources, and stop
// all publishing.
func NewEventBroker(ctx context.Context, cfg EventBrokerCfg) (*EventBroker, error) {
	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}

	// Set the event buffer size to a minimum
	if cfg.EventBufferSize == 0 {
		cfg.EventBufferSize = 100
	}

	buffer := newEventBuffer(cfg.EventBufferSize)
	e := &EventBroker{
		logger:    cfg.Logger.Named("event_broker"),
		eventBuf:  buffer,
		publishCh: make(chan *structs.Events, 64),
		aclCh:     make(chan structs.Event, 10),
		subscriptions: &subscriptions{
			byToken: make(map[string]map[*SubscribeRequest]*Subscription),
		},
	}

	go e.handleUpdates(ctx)
	go e.handleACLUpdates(ctx)

	return e, nil
}

// Len returns the current length of the event buffer.
func (e *EventBroker) Len() int {
	return e.eventBuf.Len()
}

// Publish events to all subscribers of the event Topic.
func (e *EventBroker) Publish(events *structs.Events) {
	if len(events.Events) == 0 {
		return
	}

	// Notify the broker to check running subscriptions against potentially
	// updated ACL Token or Policy
	for _, event := range events.Events {
		if event.Topic == structs.TopicACLToken || event.Topic == structs.TopicACLPolicy || event.Topic == structs.TopicACLRole {
			e.aclCh <- event
		}
	}

	e.publishCh <- events
}

// Subscribe returns a new Subscription for a given request. A Subscription
// will receive an initial empty currentItem value which points to the first item
// in the buffer. This allows the new subscription to call Next() without first checking
// for the current Item.
//
// A Subscription will start at the requested index, or as close as possible to
// the requested index if it is no longer in the buffer. If StartExactlyAtIndex is
// set and the index is no longer in the buffer or not yet in the buffer an error
// will be returned.
//
// When a caller is finished with the subscription it must call Subscription.Unsubscribe
// to free ACL tracking resources.
func (e *EventBroker) Subscribe(req *SubscribeRequest) (*Subscription, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var head *bufferItem
	var offset int
	if req.Index != 0 {
		head, offset = e.eventBuf.StartAtClosest(req.Index)
	} else {
		head = e.eventBuf.Head()
	}
	if offset > 0 && req.StartExactlyAtIndex {
		return nil, fmt.Errorf("requested index not in buffer")
	} else if offset > 0 {
		metrics.SetGauge([]string{"nomad", "event_broker", "subscription", "request_offset"}, float32(offset))
		e.logger.Debug("requested index no longer in buffer", "requsted", int(req.Index), "closest", int(head.Events.Index))
	}

	// Empty head so that calling Next on sub
	start := newBufferItem(&structs.Events{Index: req.Index})
	start.link.next.Store(head)
	close(start.link.nextCh)

	if req.Authenticate == nil {
		req.Authenticate = func() error {
			return nil
		}
	} else if err := req.Authenticate(); err != nil {
		return nil, err
	}

	sub := newSubscription(req, start, e.subscriptions.unsubscribeFn(req))

	e.subscriptions.add(req, sub)
	return sub, nil
}

// CloseAll closes all subscriptions
func (e *EventBroker) CloseAll() {
	e.subscriptions.closeAll()
}

func (e *EventBroker) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			e.subscriptions.closeAll()
			return
		case update := <-e.publishCh:
			e.eventBuf.Append(update)
		}
	}
}

func (e *EventBroker) handleACLUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-e.aclCh:
			switch payload := update.Payload.(type) {
			case *structs.ACLTokenEvent:
				tokenSecretID := payload.SecretID()

				// Token was deleted
				if update.Type == structs.TypeACLTokenDeleted {
					e.subscriptions.closeSubscriptionsForTokens([]string{tokenSecretID})
					continue
				}

				e.subscriptions.closeSubscriptionFunc(tokenSecretID, func(sub *Subscription) bool {
					return sub.req.Authenticate() != nil
				})

			case *structs.ACLPolicyEvent, *structs.ACLRoleStreamEvent:
				// Re-evaluate each subscription permission since a policy or
				// role change may alter the permissions of the token being
				// used for the subscription.
				e.checkSubscriptionsAgainstACLChange()
			}
		}
	}
}

// checkSubscriptionsAgainstACLChange iterates over the brokers subscriptions
// and evaluates whether the token used for the subscription is still valid. A
// token may become invalid is the assigned policies or roles have been updated
// which removed the required permission. If the token is no long valid, the
// subscription is closed.
func (e *EventBroker) checkSubscriptionsAgainstACLChange() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for tokenSecretID := range e.subscriptions.byToken {
		// if tokenSecretID is empty ACLs were disabled at time of subscribing
		if tokenSecretID == "" {
			continue
		}

		e.subscriptions.closeSubscriptionFunc(tokenSecretID, func(sub *Subscription) bool {
			return sub.req.Authenticate() != nil
		})
	}
}

func (s *Subscription) forceClose() {
	if atomic.CompareAndSwapUint32(&s.state, subscriptionStateOpen, subscriptionStateClosed) {
		close(s.forceClosed)
	}
}

type subscriptions struct {
	// mu for byToken. If both subscription.mu and EventBroker.mu need
	// to be held, EventBroker mutex MUST always be acquired first.
	mu sync.RWMutex

	// byToken is an mapping of active Subscriptions indexed by a token and
	// a pointer to the request.
	// When the token is modified all subscriptions under that token will be
	// reloaded.
	// A subscription may be unsubscribed by using the pointer to the request.
	byToken map[string]map[*SubscribeRequest]*Subscription
}

func (s *subscriptions) add(req *SubscribeRequest, sub *Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subsByToken, ok := s.byToken[req.Token]
	if !ok {
		subsByToken = make(map[*SubscribeRequest]*Subscription)
		s.byToken[req.Token] = subsByToken
	}
	subsByToken[req] = sub
}

func (s *subscriptions) closeSubscriptionsForTokens(tokenSecretIDs []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, secretID := range tokenSecretIDs {
		if subs, ok := s.byToken[secretID]; ok {
			for _, sub := range subs {
				sub.forceClose()
			}
		}
	}
}

func (s *subscriptions) closeSubscriptionFunc(tokenSecretID string, fn func(*Subscription) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sub := range s.byToken[tokenSecretID] {
		if fn(sub) {
			sub.forceClose()
		}
	}
}

// unsubscribeFn returns a function that the subscription will call to remove
// itself from the subsByToken.
// This function is returned as a closure so that the caller doesn't need to keep
// track of the SubscriptionRequest, and can not accidentally call unsubscribeFn with the
// wrong pointer.
func (s *subscriptions) unsubscribeFn(req *SubscribeRequest) func() {
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		subsByToken, ok := s.byToken[req.Token]
		if !ok {
			return
		}

		sub := subsByToken[req]
		if sub == nil {
			return
		}

		// close the subscription
		sub.forceClose()

		delete(subsByToken, req)
		if len(subsByToken) == 0 {
			delete(s.byToken, req.Token)
		}
	}
}

func (s *subscriptions) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, byRequest := range s.byToken {
		for _, sub := range byRequest {
			sub.forceClose()
		}
	}
}
