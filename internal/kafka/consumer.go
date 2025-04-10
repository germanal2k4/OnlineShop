package kafka

import (
	"context"
	"github.com/IBM/sarama"
	"log"
)

type ConsumerGroupHandler struct{}

func (ConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (ConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		log.Printf("Consumed message: topic=%s partition=%d offset=%d value=%s", msg.Topic, msg.Partition, msg.Offset, string(msg.Value))
		session.MarkMessage(msg, "")
	}
	return nil
}

func StartSaramaConsumer(ctx context.Context, cfg *sarama.Config, brokers []string, groupID string, topics []string) {
	config := cfg

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		log.Fatalf("Error creating consumer group: %v", err)
	}
	defer func() {
		if err := consumerGroup.Close(); err != nil {
			log.Fatalf("Error closing consumer group: %v", err)
		}
	}()

	handler := ConsumerGroupHandler{}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := consumerGroup.Consume(ctx, topics, handler); err != nil {
				log.Printf("Error from consumer: %v", err)
			}
		}
	}

}
