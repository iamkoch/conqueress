package inmemory

import (
	"fmt"
	cqrs "github.com/iamkoch/conqueress"
	"github.com/iamkoch/conqueress/eventstore"
)

type inMemoryEventDescriptor[TID comparable] struct {
	version   int
	eventData cqrs.Event
	id        TID
}

type inMemoryEventStore[TID comparable] struct {
	publisher *cqrs.Mediator
	current   map[TID][]inMemoryEventDescriptor[TID]
}

func NewInMemoryEventStore[TID comparable](m *cqrs.Mediator) eventstore.IGenericIDEventStore[TID] {
	return &inMemoryEventStore[TID]{
		m,
		make(map[TID][]inMemoryEventDescriptor[TID]),
	}
}

func (i inMemoryEventStore[TID]) SaveEvents(aggregateType string, aggregateId TID, events []cqrs.Event, expectedVersion int) error {
	eventDescriptors, ok := i.current[aggregateId]

	if !ok {
		eventDescriptors = []inMemoryEventDescriptor[TID]{}
	} else if eventDescriptors[len(eventDescriptors)-1].version != expectedVersion && expectedVersion != -1 {
		return fmt.Errorf("concurrency exception: %d != %d", eventDescriptors[len(eventDescriptors)-1].version, expectedVersion)
	}

	var ev = expectedVersion

	for _, evt := range events {
		ev++
		evt.WithVersion(ev)
		eventDescriptors = append(eventDescriptors, inMemoryEventDescriptor[TID]{
			version:   ev,
			eventData: evt,
			id:        aggregateId,
		})

		// publish
		err := i.publisher.PublishSync(evt)

		if err != nil {
			return fmt.Errorf("error publishing event: %v", err)
		}
	}

	i.current[aggregateId] = eventDescriptors

	return nil
}

func (i inMemoryEventStore[TID]) GetEventsForAggregate(aggregateId TID) []cqrs.Event {
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
