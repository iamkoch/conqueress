package conqueress

import "github.com/iamkoch/conqueress/guid"

type Event interface {
	MsgId() guid.Guid
	WithVersion(v int)
}

type BaseEvent struct {
	MessageId string `json:"message_id"`
	Ver       int    `json:"version"`
}

func (b *BaseEvent) MsgId() guid.Guid {
	return guid.MustFromString(b.MessageId)
}

func (b *BaseEvent) WithVersion(v int) {
	b.Ver = v
}

func DefaultBaseEvent() *BaseEvent {
	return &BaseEvent{Ver: -1, MessageId: guid.New().String()}
}
