package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"reflect"
	"time"
)

type simpleEnvelope struct {
	Id            guid.Guid `bson:"_id"`
	CorrelationId guid.Guid `bson:"correlation_id"`
	CausationId   guid.Guid `bson:"causation_id"`
}

type envelope struct {
	simpleEnvelope
	Type        string `bson:"type"`
	Body        string `bson:"body"`
	AggregateId string `bson:"aggregate_id"`
}

type mongoEventStore struct {
	client *mongo.Client
	tm     *TypeMap
}

type ConnectionString string

func NewMongoEventStore(cs ConnectionString, tm *TypeMap) (eventstore.IEventStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(string(cs)))
	if err != nil {
		return nil, err
	}

	return &mongoEventStore{client, tm}, nil
}

func checkConcurrency(expectedVersion int, a *dbAggregate) error {
	isNewAggregate := expectedVersion == -1
	hasVersionMismatch := a.Version != expectedVersion
	if hasVersionMismatch && !isNewAggregate {
		return errors.New("concurrency error")
	}
	return nil
}

func tryGetExistingAggregate(
	ctx mongo.SessionContext,
	ac *mongo.Collection,
	aid guid.Guid,
	createDefault func() *dbAggregate) (*dbAggregate, error) {
	agg := &dbAggregate{}
	res := ac.FindOne(ctx, bson.M{"_id": aid.String()})
	if res.Err() != nil {
		if res.Err() == mongo.ErrNoDocuments {
			return createDefault(), nil
		}
		return nil, res.Err()
	}

	e := res.Decode(agg)

	if e != nil {
		return nil, e
	}

	return agg, nil
}

func (m mongoEventStore) SaveEvents(aggregateType string, aggregateId guid.Guid, events []cqrs.Event, expectedVersion int) error {
	ec := m.client.Database("devly").Collection("events")
	ac := m.client.Database("devly").Collection("aggregates")

	session, err := m.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(context.Background())

	wc := writeconcern.New(writeconcern.WMajority())
	rc := readconcern.Snapshot()
	txnOpts := options.Transaction().SetWriteConcern(wc).SetReadConcern(rc)

	err = mongo.WithSession(context.Background(), session, func(sessionContext mongo.SessionContext) error {
		if err = session.StartTransaction(txnOpts); err != nil {
			return err
		}

		getDefaultAggregate := func() *dbAggregate {
			return &dbAggregate{Id: aggregateId.String(), Version: 0}
		}

		dbAgg, e := tryGetExistingAggregate(sessionContext, ac, aggregateId, getDefaultAggregate)

		if e != nil {
			return e
		}

		e = checkConcurrency(expectedVersion, dbAgg)

		if e != nil {
			fmt.Println("concurrency error")
			return e
		}

		ev := expectedVersion

		for _, event := range events {
			ev++
			dbe, e := createDbEvent(event, aggregateType, guid.New(), guid.New(), aggregateId, ev)
			if e != nil {
				return e
			}

			_, e = ec.InsertOne(sessionContext, dbe)

			if e != nil {
				fmt.Println("Error saving event ", e)
				return e
			}
		}

		_, e = ac.UpdateOne(sessionContext, bson.M{"_id": aggregateId.String()}, bson.M{"$set": bson.M{"version": ev}}, options.Update().SetUpsert(true))

		if err = session.CommitTransaction(sessionContext); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error calling transaction %s\n", err.Error())
		return err
	} else {
		fmt.Printf("Transaction successful\n")
	}
	return nil
}

func (m mongoEventStore) GetEventsForAggregate(aggregateId guid.Guid) []cqrs.Event {
	ec := m.client.Database("devly").Collection("events")
	c, e := ec.Find(context.Background(), bson.M{"aggregate_id": aggregateId.String()})
	if e != nil {
		//if e.Error() == mongo.ErrNoDocuments {
		//	return []cqrs.Event{}
		//}
		panic(e.Error())
	}

	var results []envelope
	if err := c.All(context.TODO(), &results); err != nil {
		panic(err)
	}

	events := make([]cqrs.Event, 0)
	for _, envelope := range results {
		get, e := m.tm.Get(envelope.Type)
		if e != nil {
			fmt.Println("Error getting event ", e)
			panic("couldn't get event")
		}
		ev, err := envelopeToEvent(get, &envelope)
		if err != nil {
			fmt.Println("Error getting event ", err)
			panic("couldn't get event")
		}
		events = append(events, ev)
	}

	return events
}

func envelopeToEvent(t reflect.Type, e *envelope) (cqrs.Event, error) {
	v := reflect.New(t)

	// reflected pointer
	newP := v.Interface()

	// Unmarshal to reflected struct pointer
	json.Unmarshal([]byte(e.Body), newP)

	return dereferenceIfPtr(newP).(cqrs.Event), nil
}

func dereferenceIfPtr(value interface{}) interface{} {
	if reflect.TypeOf(value).Kind() == reflect.Ptr {

		return reflect.ValueOf(value).Elem().Interface()

	} else {

		return value

	}
}

type dbEvent struct {
	Id            string `bson:"id"`
	AggregateId   string `bson:"aggregate_id"`
	AggregateType string `bson:"aggregate_type"`
	Body          string `bson:"body"`
	Type          string `bson:"type"`
	Version       int    `bson:"version"`
	Timestamp     int64  `bson:"timestamp"`
	CorrelationId string `bson:"correlation_id"`
	CausationId   string `bson:"causation_id"`
}

type dbAggregate struct {
	Id      string `bson:"_id"`
	Version int    `bson:"version"`
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
