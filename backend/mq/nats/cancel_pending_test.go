package nats

import (
	"os"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/nats-io/nats.go"
)

func TestVectorCancellationBoundaryMatchesRealMessageMetadata(t *testing.T) {
	address := os.Getenv("PANDA_WIKI_TEST_ISOLATED_NATS_URL")
	if address == "" {
		t.Skip("isolated disposable NATS required")
	}
	cfg := &config.Config{MQ: config.MQConfig{NATS: config.NATSConfig{Server: address}}}
	p, err := NewMQProducer(cfg, log.NewLogger(cfg))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Produce(t.Context(), domain.VectorTaskTopic, "", []byte("old")); err != nil {
		t.Fatal(err)
	}
	boundary, err := p.VectorQueueBoundary(t.Context())
	if err != nil || boundary.Sequence == 0 || boundary.PublishedAt.IsZero() {
		t.Fatal(boundary, err)
	}
	if err := p.Produce(t.Context(), domain.VectorTaskTopic, "", []byte("new")); err != nil {
		t.Fatal(err)
	}
	sub, err := p.js.SubscribeSync(domain.VectorTaskTopic, nats.Durable("cancel-boundary-test"), nats.DeliverAll(), nats.ManualAck())
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	for _, canceled := range []bool{true, false} {
		msg, err := sub.NextMsg(3 * time.Second)
		if err != nil {
			t.Fatal(err)
		}
		seq, at, err := (&Message{msg: msg}).VectorQueuePosition()
		if err != nil {
			t.Fatal(err)
		}
		matches := seq <= boundary.Sequence && !at.After(boundary.PublishedAt)
		if matches != canceled {
			t.Fatalf("queue boundary misclassified actual NATS message seq=%d at=%v cutoff=%+v", seq, at, boundary)
		}
		if err := msg.AckSync(); err != nil {
			t.Fatal(err)
		}
	}
}
