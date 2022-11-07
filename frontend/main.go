package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/Shopify/sarama"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"sync"
	"unicode/utf8"
)

//go:embed static
var static embed.FS

func main() {
	listenAddress := flag.String("l", ":8000", "Listen address of the server")
	kafkaAddress := flag.String("k", "localhost:9092", "Kafka instance adress")
	flag.Parse()

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

	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	staticRoot, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}

	http.Handle("/", http.FileServer(http.FS(staticRoot)))
	http.Handle("/message-get", messageGetter(client))
	http.HandleFunc("/message-send", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else if !utf8.Valid(data) {
			http.Error(w, "Invalid utf-8 messgage\n", http.StatusBadRequest)
			return
		}
		_, _, err = producer.SendMessage(&sarama.ProducerMessage{
			Topic: "chatt-input",
			Value: sarama.StringEncoder(string(data)),
		})
		if err != nil {
			http.Error(w, "Intern error\r\n", http.StatusInternalServerError)
			return
		}
	})

	log.Println("Listen", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func messageGetter(client sarama.Client) http.HandlerFunc {
	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		log.Fatal(err)
	}
	partitionConsumer, err := consumer.ConsumePartition("chatt-output", 0, sarama.OffsetOldest)
	if err != nil {
		log.Fatal(err)
	}

	cond := sync.NewCond(&sync.Mutex{})
	allMessages := make([]string, 0) // First message index is 1, not zero.
	go func() {
		for message := range partitionConsumer.Messages() {
			allMessages = append(allMessages, string(message.Value))
			log.Printf("%q", message.Value)
			cond.Broadcast()
		}
	}()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("retry: 500\n\n"))

		lastEventID, _ := strconv.Atoi(r.Header.Get("Last-Event-Id"))
		if lastEventID < 0 || len(allMessages) <= lastEventID {
			lastEventID = 0
		}

		for i, message := range allMessages[lastEventID+1:] {
			fmt.Fprintf(w, "data: %s\nid: %d\n\n", message, i+lastEventID+1)
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		for i := len(allMessages); ; i++ {
			flusher.Flush()

			cond.L.Lock()
			cond.Wait()
			cond.L.Unlock()

			_, err := fmt.Fprintf(w, "data: %s\nid: %d\n\n", allMessages[i], i+1)
			if err != nil {
				return
			}
		}
	}
}
