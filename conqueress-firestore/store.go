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

func tryGetExistingAggregate(
	ctx context.Context,
	ac *firestore.CollectionRef,
	aid guid.Guid,
	createDefault func() *dbAggregate) (*dbAggregate, error) {
	a, e := ac.Doc(aid.String()).Get(ctx)
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
		dbAgg, e := tryGetExistingAggregate(ctx, ac, aggregateId, getDefaultAggregate)

		e = checkConcurrency(expectedVersion, dbAgg)

		if e != nil {
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

func NewFirestoreEventStore(ctx context.Context, tm *TypeMap) (eventstore.IEventStore, error) {
	client, err := firestore.NewClient(ctx, "iamkoch")

	if err != nil {
		fmt.Println("Error creating client ", err)
		return nil, err
	}

	return firestoreEventStore{client, tm}, nil
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
