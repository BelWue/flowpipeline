package kafkaconsumer

import (
	"testing"
)

func TestSegment_KafkaConsumer_instanciation(t *testing.T) {
	kafkaConsumer := &KafkaConsumer{}
	result := kafkaConsumer.New(map[string]string{})
	if result != nil {
		t.Error("([error] Segment KafkaConsumer intiated successfully despite bad base config.")
	}

	result = kafkaConsumer.New(map[string]string{"server": "doh", "topic": "duh", "group": "yolo", "tls": "1"})
	if result != nil {
		t.Error("([error] Segment KafkaConsumer intiated successfully despite bad auth config.")
	}

	result = kafkaConsumer.New(map[string]string{"server": "doh", "topic": "duh", "group": "yolo", "tls": "4", "auth": "maybe"})
	if result != nil {
		t.Error("([error] Segment KafkaConsumer intiated successfully despite bad booleans in config.")
	}

	result = kafkaConsumer.New(map[string]string{"server": "doh", "topic": "duh", "group": "yolo", "auth": "0"})
	if result == nil {
		t.Error("([error] Segment KafkaConsumer did not initiate successfully.")
	}
}
