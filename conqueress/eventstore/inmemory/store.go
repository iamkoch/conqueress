package inmemory

import (
	"fmt"
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
	"github.com/iamkoch/conqueress/guid"
)

type inMemoryEventDescriptor struct {
	version   int
	eventData cqrs.Event
	id        guid.Guid
}

type inMemoryEventStore struct {
	publisher *cqrs.Mediator
	current   map[guid.Guid][]inMemoryEventDescriptor
}

func NewInMemoryEventStore(m *cqrs.Mediator) eventstore.IEventStore {
	return &inMemoryEventStore{
		m,
		make(map[guid.Guid][]inMemoryEventDescriptor),
	}
}

func (i inMemoryEventStore) SaveEvents(aggregateType string, aggregateId guid.Guid, events []cqrs.Event, expectedVersion int) error {
	eventDescriptors, ok := i.current[aggregateId]

	if !ok {
		eventDescriptors = []inMemoryEventDescriptor{}
	} else if eventDescriptors[len(eventDescriptors)-1].version != expectedVersion && expectedVersion != -1 {
		return fmt.Errorf("concurrency exception: %d != %d", eventDescriptors[len(eventDescriptors)-1].version, expectedVersion)
	}

	var ev = expectedVersion

	for _, evt := range events {
		ev++
		evt.WithVersion(ev)
		eventDescriptors = append(eventDescriptors, inMemoryEventDescriptor{
			version:   ev,
			eventData: evt,
			id:        aggregateId,
		})

		// publish
		err := i.publisher.Publish(evt)

		if err != nil {
			return fmt.Errorf("error publishing event: %v", err)
		}
	}

	i.current[aggregateId] = eventDescriptors

	return nil
}

func (i inMemoryEventStore) GetEventsForAggregate(aggregateId guid.Guid) []cqrs.Event {
	eventDescriptors, ok := i.current[aggregateId]
	evs := make([]cqrs.Event, 0)
	if !ok {
		return evs
	}

	for _, d := range eventDescriptors {
		evs = append(evs, d.eventData)
	}

	return evs
}
