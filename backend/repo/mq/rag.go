package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/mq"
)

type RAGRepository struct {
	producer mq.MQProducer
}

func (r *RAGRepository) VectorQueueBoundary(ctx context.Context) (domain.VectorQueueBoundary, error) {
	p, ok := r.producer.(interface {
		VectorQueueBoundary(context.Context) (domain.VectorQueueBoundary, error)
	})
	if !ok {
		return domain.VectorQueueBoundary{}, fmt.Errorf("当前消息队列不支持安全取消旧任务")
	}
	return p.VectorQueueBoundary(ctx)
}

func NewRAGRepository(producer mq.MQProducer) *RAGRepository {
	return &RAGRepository{producer: producer}
}

func (r *RAGRepository) AsyncUpdateNodeReleaseVector(ctx context.Context, request []*domain.NodeReleaseVectorRequest) error {
	for _, req := range request {
		requestBytes, err := json.Marshal(req)
		if err != nil {
			return err
		}
		if err := r.producer.Produce(ctx, domain.VectorTaskTopic, "", requestBytes); err != nil {
			return err
		}
	}
	return nil
}
