// This file defines the function-type aliases used for typed handler
// composition in the style of IntelAgent.Framework's HandleCommand<T>
// delegate.
//
// The mediator gives you reflection-based dispatch by Go type. These typed
// function aliases let you compose handlers via factory functions that close
// over their dependencies — useful when you don't need a central dispatcher
// and just want statically-typed command and event handling at the
// application-layer boundary.
package conqueress

import (
	"context"

	"github.com/iamkoch/conqueress/guid"
)

// HandleCommand is a typed command-handler function. A factory in the
// application layer composes one of these by closing over its dependencies
// (repository, ACLs, logger, etc.):
//
//	func MakeCreatePolicyHandler(repo PolicyRepo, log Logger) conqueress.HandleCommand[CreatePolicy] {
//	    return func(ctx context.Context, cmd CreatePolicy, correlation, causation guid.Guid) error {
//	        log.Info("creating policy", "correlation", correlation)
//	        policy := NewPolicy(cmd)
//	        return repo.Save(ctx, policy, -1, correlation, causation)
//	    }
//	}
//
// The HTTP-command-handler module receives the typed handler at construction
// time and calls it with the parsed command from the request body.
type HandleCommand[T any] func(ctx context.Context, cmd T, correlationId, causationId guid.Guid) error

// HandleQuery is a typed query-handler function. Symmetric with HandleCommand
// but returns a result and carries no causation (queries don't cause domain
// changes; they read state).
type HandleQuery[Q any, R any] func(ctx context.Context, query Q, correlationId guid.Guid) (R, error)

// HandleEvent is a typed event-handler function. An aggregate emits a domain
// event; an in-process subscriber (an ACL, a projection updater) consumes it
// via one of these. Distinct from the EventProcessor type used by the
// reflection-based Mediator: HandleEvent[T] is statically typed.
type HandleEvent[T Event] func(ctx context.Context, evt T, correlationId guid.Guid) error

// IntegrationEventPublisher publishes integration events onto the cross-
// service message bus. The integrationevents ACL implements this; the
// application layer depends on the interface, not a concrete bus.
type IntegrationEventPublisher interface {
	Publish(ctx context.Context, evt IntegrationEvent, correlationId, causationId guid.Guid) error
}
