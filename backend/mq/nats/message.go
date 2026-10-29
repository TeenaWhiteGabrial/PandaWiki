package nats

import (
	"github.com/nats-io/nats.go"
	"time"

	"github.com/chaitin/panda-wiki/mq/types"
)

func (m *Message) VectorQueuePosition() (uint64, time.Time, error) {
	metadata, err := m.msg.Metadata()
	if err != nil {
		return 0, time.Time{}, err
	}
	return metadata.Sequence.Stream, metadata.Timestamp, nil
}

type Message struct {
	msg *nats.Msg
}

func (m *Message) GetData() []byte {
	return m.msg.Data
}

func (m *Message) GetTopic() string {
	return m.msg.Subject
}

var _ types.Message = (*Message)(nil)
