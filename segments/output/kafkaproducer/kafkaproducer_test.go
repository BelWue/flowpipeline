package kafkaproducer

import (
	"testing"
)

func TestSegment_KafkaProducer_instanciation(t *testing.T) {
	kafkaProducer := &KafkaProducer{}
	result := kafkaProducer.New(map[string]string{})
	if result != nil {
		t.Error("([error] Segment KafkaProducer intiated successfully despite bad base config.")
	}

	result = kafkaProducer.New(map[string]string{"server": "doh", "topic": "duh", "tls": "1"})
	if result != nil {
		t.Error("([error] Segment KafkaProducer intiated successfully despite bad auth config.")
	}

	result = kafkaProducer.New(map[string]string{"server": "doh", "topic": "duh", "auth": "f", "topicsuffix": "Bytes"})
	if result == nil {
		t.Error("([error] Segment KafkaProducer did not intiate successfully despite good topicsuffix config.")
	}

	result = kafkaProducer.New(map[string]string{"server": "doh", "topic": "duh", "topicsuffix": "Meh"})
	if result != nil {
		t.Error("([error] Segment KafkaProducer intiated successfully despite bad topicsuffix config.")
	}

	result = kafkaProducer.New(map[string]string{"server": "doh", "topic": "duh", "tls": "4", "auth": "maybe"})
	if result != nil {
		t.Error("([error] Segment KafkaProducer intiated successfully despite bad booleans in config.")
	}

	result = kafkaProducer.New(map[string]string{"server": "doh", "topic": "duh", "auth": "0"})
	if result == nil {
		t.Error("([error] Segment KafkaProducer did not initiate successfully.")
	}
}
