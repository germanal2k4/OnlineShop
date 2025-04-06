package kafka

import (
	"log"
	"time"

	"github.com/IBM/sarama"
)

type SaramaProducer struct {
	producer sarama.SyncProducer
}

func NewSaramaProducer(brokers []string) (*SaramaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Timeout = 5 * time.Second
	prod, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}
	return &SaramaProducer{producer: prod}, nil
}

func (p *SaramaProducer) Publish(topic string, message []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}
	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		log.Printf("Failed to send message to topic %s: %v", topic, err)
		return err
	}
	log.Printf("Message stored in topic(%s)/partition(%d)/offset(%d)", topic, partition, offset)
	return nil
}

func (p *SaramaProducer) Close() error {
	return p.producer.Close()
}
