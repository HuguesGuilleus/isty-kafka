package main

import (
	"flag"
	"github.com/Shopify/sarama"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

func main() {
	kafkaAddress := flag.String("k", "localhost:9092", "Kafka instance adress")
	flag.Parse()

	// Init client
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Partitioner = sarama.NewHashPartitioner
	client, err := sarama.NewClient([]string{*kafkaAddress}, config)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Create producer
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	// Create consumer
	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()
	partitionConsumer, err := consumer.ConsumePartition("chatt-input", 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatal(err)
	}
	defer partitionConsumer.Close()

	// Make stream
	for message := range partitionConsumer.Messages() {
		value := transformMessage(message.Timestamp, message.Value)
		if value == "" {
			continue
		}

		_, _, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: "chatt-output",
			Value: sarama.StringEncoder(value),
		})
		if err != nil {
			log.Println("Send message fail:", err)
			return
		}
	}
}

var spaces = regexp.MustCompile(`\s+`)

func transformMessage(timestamp time.Time, messageBytes []byte) string {
	if !utf8.Valid(messageBytes) {
		return ""
	}

	message := strings.TrimSpace(string(messageBytes))
	message = spaces.ReplaceAllString(message, " ")
	message = strings.ToLower(message)

	if message == "" {
		return ""
	}
	return timestamp.UTC().Format("[2006-01-02 15:04:05 UTC] ") + message
}
