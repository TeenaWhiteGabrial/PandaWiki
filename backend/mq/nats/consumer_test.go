package nats

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/mq/types"
	"github.com/nats-io/nats.go"
)

func testJetStream(t *testing.T) (*nats.Conn, nats.JetStreamContext, string, string, string) {
	t.Helper()
	address := os.Getenv("PANDA_WIKI_TEST_NATS_URL")
	if address == "" {
		t.Skip("set PANDA_WIKI_TEST_NATS_URL for queue integration")
	}
	conn, err := nats.Connect(address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	js, err := conn.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("pdf_test_%d", time.Now().UnixNano())
	topic, consumer := name+".task", name+"_consumer"
	if _, err := js.AddStream(&nats.StreamConfig{Name: name, Subjects: []string{topic}, Storage: nats.MemoryStorage}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { js.DeleteStream(name) })
	if _, err := js.AddConsumer(name, &nats.ConsumerConfig{Durable: consumer, DeliverSubject: nats.NewInbox(), FilterSubject: topic, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy, AckWait: 200 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	return conn, js, name, topic, consumer
}

func TestFailedHandlerRemainsUnacknowledged(t *testing.T) {
	conn, js, _, topic, consumer := testJetStream(t)
	domain.TopicConsumerName[topic] = consumer
	t.Cleanup(func() { delete(domain.TopicConsumerName, topic) })
	c := &MQConsumer{conn: conn, js: js, handlers: make(map[string]*nats.Subscription), logger: log.NewLogger(&config.Config{})}
	var attempts atomic.Int32
	done := make(chan struct{}, 1)
	if err := c.RegisterHandler(topic, func(context.Context, types.Message) error {
		if attempts.Add(1) == 1 {
			return fmt.Errorf("transient failure")
		}
		select {
		case done <- struct{}{}:
		default:
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	defer c.handlers[topic].Unsubscribe()
	if _, err := js.Publish(topic, []byte("synthetic PDF task")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("failed handler was automatically acknowledged instead of retried")
	}
}

func TestHeartbeatPreventsRedeliveryDuringLongParsing(t *testing.T) {
	_, js, _, topic, consumer := testJetStream(t)
	var attempts atomic.Int32
	done := make(chan struct{}, 1)
	sub, err := js.Subscribe(topic, func(message *nats.Msg) {
		attempts.Add(1)
		stop := keepMessageInProgress(message, 40*time.Millisecond)
		time.Sleep(600 * time.Millisecond)
		stop()
		message.Ack()
		select {
		case done <- struct{}{}:
		default:
		}
	}, nats.ManualAck(), nats.Durable(consumer))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	if _, err := js.Publish(topic, []byte("synthetic long OCR task")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler never finished")
	}
	time.Sleep(300 * time.Millisecond)
	if attempts.Load() != 1 {
		t.Fatalf("long parsing delivered %d times", attempts.Load())
	}
}
