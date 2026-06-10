// Package mongo provides a MongoDB-backed event store implementing the
// context-aware eventstore.Store[TID] port.
//
// Layout (two collections, transactional append):
//
//	<collection>           one document per event:
//	    _id            event message id
//	    aggregate_id   aggregate id (TID rendered via String())
//	    aggregate_type aggregate type name
//	    version        stream position (0-based)
//	    type           wire-stable event name (TypeRegistry)
//	    body           JSON payload
//	    correlation_id / causation_id / occurred_at   event metadata
//	    recorded_at    server wall clock at append
//
//	<collection>_streams   one document per stream, for optimistic
//	    _id            aggregate id                  concurrency:
//	    version        current stream version
//
// Transactions require a MongoDB replica set (a single-node replica set is
// fine for local development).
//
// Publishing to a message bus is deliberately NOT done here. Publish after a
// successful Save in your application layer; keeping the store pure makes the
// transaction boundary obvious.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
	"go.mongodb.org/mongo-driver/bson"
	driver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type envelope struct {
	ID            string    `bson:"_id"`
	AggregateID   string    `bson:"aggregate_id"`
	AggregateType string    `bson:"aggregate_type"`
	Version       int       `bson:"version"`
	Type          string    `bson:"type"`
	Body          []byte    `bson:"body"`
	CorrelationID string    `bson:"correlation_id,omitempty"`
	CausationID   string    `bson:"causation_id,omitempty"`
	OccurredAt    time.Time `bson:"occurred_at,omitempty"`
	RecordedAt    time.Time `bson:"recorded_at"`
}

type streamHead struct {
	ID      string `bson:"_id"`
	Version int    `bson:"version"`
}

// Stringer is the constraint on aggregate IDs: anything that renders itself
// to a stable string. Composite IDs implement String() to produce a stable,
// unique rendering.
type Stringer interface {
	String() string
}

// Store is a MongoDB event store generic over the aggregate ID type.
// Satisfies eventstore.Store[TID].
type Store[TID Stringer] struct {
	client   *driver.Client
	registry *TypeRegistry

	events  *driver.Collection
	streams *driver.Collection
}

// NewStore wires a Store onto a database and base collection name (events in
// <collection>, stream heads in <collection>_streams).
func NewStore[TID Stringer](client *driver.Client, database, collection string, registry *TypeRegistry) *Store[TID] {
	db := client.Database(database)
	return &Store[TID]{
		client:   client,
		registry: registry,
		events:   db.Collection(collection),
		streams:  db.Collection(collection + "_streams"),
	}
}

// SaveEvents appends the events at expectedVersion (-1 for a new stream),
// assigning stream positions via WithVersion. The stream-head check and the
// event inserts run in one transaction; a concurrent writer loses cleanly
// with eventstore.ErrConcurrencyException.
func (s *Store[TID]) SaveEvents(
	ctx context.Context,
	aggregateType string,
	aggregateId TID,
	evts []conqueress.Event,
	expectedVersion int,
) error {
	if len(evts) == 0 {
		return nil
	}

	session, err := s.client.StartSession()
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}
	defer session.EndSession(ctx)

	txnOpts := options.Transaction().
		SetWriteConcern(writeconcern.Majority()).
		SetReadConcern(readconcern.Snapshot())

	_, err = session.WithTransaction(ctx, func(sc driver.SessionContext) (any, error) {
		current := -1
		var head streamHead
		findErr := s.streams.FindOne(sc, bson.M{"_id": aggregateId.String()}).Decode(&head)
		switch {
		case findErr == nil:
			current = head.Version
		case errors.Is(findErr, driver.ErrNoDocuments):
			// new stream, current stays -1
		default:
			return nil, fmt.Errorf("load stream head: %w", findErr)
		}

		if current != expectedVersion {
			return nil, fmt.Errorf("%w: stream %s at %d, expected %d",
				eventstore.ErrConcurrencyException, aggregateId.String(), current, expectedVersion)
		}

		version := expectedVersion
		docs := make([]any, 0, len(evts))
		now := time.Now().UTC()
		for _, e := range evts {
			version++
			e.WithVersion(version)
			name, body, encErr := s.registry.Encode(e)
			if encErr != nil {
				return nil, encErr
			}
			docs = append(docs, envelope{
				ID:            e.MsgId().String(),
				AggregateID:   aggregateId.String(),
				AggregateType: aggregateType,
				Version:       version,
				Type:          name,
				Body:          body,
				CorrelationID: guidOrEmpty(e.CorrelationId()),
				CausationID:   guidOrEmpty(e.CausationId()),
				OccurredAt:    e.OccurredAt(),
				RecordedAt:    now,
			})
		}

		if _, insErr := s.events.InsertMany(sc, docs); insErr != nil {
			return nil, fmt.Errorf("insert events: %w", insErr)
		}

		_, upErr := s.streams.UpdateOne(sc,
			bson.M{"_id": aggregateId.String()},
			bson.M{"$set": bson.M{"version": version}},
			options.Update().SetUpsert(true))
		if upErr != nil {
			return nil, fmt.Errorf("advance stream head: %w", upErr)
		}
		return nil, nil
	}, txnOpts)

	return err
}

// GetEventsForAggregate loads the stream in version order. An empty slice
// (no error) means the stream does not exist; eventstore.ContextRepository
// maps that to ErrAggregateNotFound.
func (s *Store[TID]) GetEventsForAggregate(ctx context.Context, aggregateId TID) ([]conqueress.Event, error) {
	cur, err := s.events.Find(ctx,
		bson.M{"aggregate_id": aggregateId.String()},
		options.Find().SetSort(bson.M{"version": 1}))
	if err != nil {
		return nil, fmt.Errorf("find events: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []conqueress.Event
	for cur.Next(ctx) {
		var env envelope
		if decErr := cur.Decode(&env); decErr != nil {
			return nil, fmt.Errorf("decode envelope: %w", decErr)
		}
		evt, decErr := s.registry.Decode(env.Type, env.Body)
		if decErr != nil {
			return nil, decErr
		}
		out = append(out, evt)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("cursor: %w", err)
	}
	return out, nil
}

// EnsureIndexes creates the indexes the store relies on. Run once at service
// startup.
func (s *Store[TID]) EnsureIndexes(ctx context.Context) error {
	_, err := s.events.Indexes().CreateOne(ctx, driver.IndexModel{
		Keys:    bson.D{{Key: "aggregate_id", Value: 1}, {Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("IX_aggregate_version"),
	})
	if err != nil {
		return fmt.Errorf("create event-stream index: %w", err)
	}
	return nil
}

// guidOrEmpty renders a guid, mapping the empty guid to "" so absent metadata
// persists as an absent BSON field rather than a zero id.
func guidOrEmpty(g guid.Guid) string {
	if g == guid.Empty {
		return ""
	}
	return g.String()
}
