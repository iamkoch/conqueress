package store

import (
	"cloud.google.com/go/firestore"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"reflect"
	"sort"
	"time"
)

type SimpleEnvelope struct {
	Id            string `firestore:"id"`
	CorrelationId string `firestore:"correlation_id"`
	CausationId   string `firestore:"causation_id"`
}

// Envelope TODO: move
type Envelope struct {
	SimpleEnvelope
	Type        string `firestore:"type"`
	Body        string `firestore:"body"`
	AggregateId string `firestore:"aggregate_id"`
}

func createDbEvent(
	e cqrs.Event,
	aggName string,
	cor guid.Guid,
	cau guid.Guid,
	aid guid.Guid,
	v int) (*dbEvent, error) {
	bytes, err := json.Marshal(e)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &dbEvent{
		Id:            e.MsgId().String(),
		AggregateId:   aid.String(),
		AggregateType: aggName,
		Body:          string(bytes),
		Type:          reflect.TypeOf(e).Name(),
		Version:       v,
		Timestamp:     time.Now().UTC().Unix(),
		CorrelationId: cor.String(),
		CausationId:   cau.String(),
	}, nil
}

func createEnvelope(
	e cqrs.Event,
	cor guid.Guid,
	cau guid.Guid,
	aid guid.Guid) (*Envelope, error) {
	b, err := json.Marshal(e)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &Envelope{
		SimpleEnvelope: SimpleEnvelope{
			Id:            e.MsgId().String(),
			CorrelationId: cor.String(),
			CausationId:   cau.String(),
		},
		Type:        reflect.TypeOf(e).Name(),
		Body:        string(b),
		AggregateId: aid.String(),
	}, nil
}

type firestoreEventStore struct {
	client *firestore.Client
	tm     *TypeMap
}

func dereferenceIfPtr(value interface{}) interface{} {
	if reflect.TypeOf(value).Kind() == reflect.Ptr {

		return reflect.ValueOf(value).Elem().Interface()

	} else {

		return value

	}
}

func envelopeToEvent(t reflect.Type, e *dbEvent) (cqrs.Event, error) {
	v := reflect.New(t)

	// reflected pointer
	newP := v.Interface()

	// Unmarshal to reflected struct pointer
	json.Unmarshal([]byte(e.Body), newP)

	event := dereferenceIfPtr(newP).(cqrs.Event)
	event.WithVersion(e.Version)
	return event, nil
}

type dbEvent struct {
	Id            string `firestore:"id"`
	AggregateId   string `firestore:"aggregate_id"`
	AggregateType string `firestore:"aggregate_type"`
	Body          string `firestore:"body"`
	Type          string `firestore:"type"`
	Version       int    `firestore:"version"`
	Timestamp     int64  `firestore:"timestamp"`
	CorrelationId string `firestore:"correlation_id"`
	CausationId   string `firestore:"causation_id"`
}

type dbAggregate struct {
	Id      string `firestore:"id"`
	Version int    `firestore:"version"`
	IsNew   bool   `firestore:"-"`
}

// tryGetExistingAggregate reads the aggregate through the transaction, so its
// version joins the transaction's read set and a concurrent write to the same
// aggregate aborts this one. A plain DocumentRef.Get does not join the read
// set, which leaves checkConcurrency comparing against a version that another
// writer may already have moved.
func tryGetExistingAggregate(
	transaction *firestore.Transaction,
	ac *firestore.CollectionRef,
	aid guid.Guid,
	createDefault func() *dbAggregate) (*dbAggregate, error) {
	a, e := transaction.Get(ac.Doc(aid.String()))
	if e != nil && status.Code(e) == codes.NotFound {
		return createDefault(), nil
	}

	if e != nil {
		return nil, e
	}

	var ai dbAggregate
	if a != nil {
		if e = a.DataTo(&ai); e != nil {
			return nil, e
		}
	} else {
		ai = *createDefault()
	}
	return &ai, nil
}

func (f firestoreEventStore) SaveEvents(aggName string, aggregateId guid.Guid, events []cqrs.Event, expectedVersion int) error {
	ec := f.client.Collection("events")
	ac := f.client.Collection("aggregates")

	err := f.client.RunTransaction(context.TODO(), func(ctx context.Context, transaction *firestore.Transaction) error {

		getDefaultAggregate := func() *dbAggregate {
			return &dbAggregate{Id: aggregateId.String(), Version: 0, IsNew: true}
		}
		dbAgg, e := tryGetExistingAggregate(transaction, ac, aggregateId, getDefaultAggregate)
		if e != nil {
			return e
		}

		if e := checkConcurrency(expectedVersion, dbAgg); e != nil {
			return eventstore.ErrConcurrencyException
		}

		ev := expectedVersion

		for _, event := range events {
			ev++
			dbe, e := createDbEvent(event, aggName, guid.New(), guid.New(), aggregateId, ev)
			if e != nil {
				return e
			}

			e = transaction.Set(ec.Doc(event.MsgId().String()), dbe)
			if e != nil {
				fmt.Println("Error saving event ", e)
				return e
			}
		}

		dbAgg.Version = ev
		e = transaction.Set(ac.Doc(aggregateId.String()), dbAgg)

		return e
	})

	if err != nil {
		fmt.Printf("Error calling transaction %s\n", err.Error())
		return err
	} else {
		fmt.Printf("Transaction successful\n")
	}
	return nil
}

func checkConcurrency(expectedVersion int, a *dbAggregate) error {
	if aggNil := a == nil; aggNil {
		return fmt.Errorf("aggregate is nil")
	}

	if a.IsNew {
		if expectedVersion == -1 {
			return nil
		} else {
			return fmt.Errorf("aggregate is new but version doesn't support expectation")
		}
	}

	hasVersionMismatch := a.Version != expectedVersion
	if hasVersionMismatch {
		return errors.New("concurrency error")
	}
	return nil
}

func (f firestoreEventStore) GetEventsForAggregate(aggregateId guid.Guid) []cqrs.Event {
	ec := f.client.Collection("events")
	q := ec.Query.Where("aggregate_id", "==", aggregateId.String())
	iter := q.Documents(context.TODO())
	defer iter.Stop()

	envelopes := make([]dbEvent, 0)

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}

		if err != nil {
			fmt.Println("Error getting document ", err)
			panic("couldn't get doc")
		}

		var env dbEvent

		if err = doc.DataTo(&env); err != nil {
			fmt.Println("Error getting data ", err)
			panic("couldn't get data")
		}

		envelopes = append(envelopes, env)

	}

	// Firestore returns query results in an unspecified order, so restore the
	// order the events were written in before replaying them.
	sort.Slice(envelopes, func(i, j int) bool {
		return envelopes[i].Version < envelopes[j].Version
	})

	events := make([]cqrs.Event, 0)
	for _, env := range envelopes {
		ev, err := envelopeToEvent(f.tm.Get(env.Type), &env)
		if err != nil {
			fmt.Println("Error getting event ", err)
			panic("couldn't get event")
		}
		events = append(events, ev)
	}

	return events

}

// NewFirestoreEventStore opens a client against the Firestore project named by
// projectID. Pass firestore.DetectProjectID to take the project from the
// environment instead, which reads GOOGLE_CLOUD_PROJECT and then the
// credentials the process is running under.
//
// The store owns the client it opens and there is no way to close it. Use
// NewFirestoreEventStoreWithClient if the client's lifetime matters to you.
func NewFirestoreEventStore(ctx context.Context, projectID string, tm *TypeMap) (eventstore.IEventStore, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("creating firestore client for project %q: %w", projectID, err)
	}

	return firestoreEventStore{client, tm}, nil
}

// NewFirestoreEventStoreWithClient wraps a client you have already configured,
// for the client options this package does not expose. You own the client and
// must close it.
func NewFirestoreEventStoreWithClient(client *firestore.Client, tm *TypeMap) eventstore.IEventStore {
	return firestoreEventStore{client, tm}
}

type TypeMap struct {
	typeMap map[string]reflect.Type
}

func (tm *TypeMap) Get(t string) reflect.Type {
	return tm.typeMap[t]
}

func NewTypeMap() *TypeMap {
	return &TypeMap{typeMap: make(map[string]reflect.Type)}
}

func (tm *TypeMap) Add(t any) *TypeMap {
	tm.typeMap[reflect.TypeOf(t).Name()] = reflect.TypeOf(t)
	return tm
}
