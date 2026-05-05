package processor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// WorkerPool manages a pool of goroutines that consume from a Redis stream
// using consumer groups (XREADGROUP). Each message is processed by exactly
// one worker and acknowledged (XACK) after successful processing.
type WorkerPool struct {
	redis         *iredis.Client
	debouncer     *Debouncer
	streamName    string
	consumerGroup string
	namePrefix    string
	workerCount   int
	stopCh        chan struct{}
}

// NewWorkerPool creates the pool and ensures the consumer group exists.
func NewWorkerPool(
	redisClient *iredis.Client,
	debouncer *Debouncer,
	streamName string,
	consumerGroup string,
	namePrefix string,
	workerCount int,
) (*WorkerPool, error) {
	wp := &WorkerPool{
		redis:         redisClient,
		debouncer:     debouncer,
		streamName:    streamName,
		consumerGroup: consumerGroup,
		namePrefix:    namePrefix,
		workerCount:   workerCount,
		stopCh:        make(chan struct{}),
	}

	// Create the consumer group. If it already exists, ignore the error.
	// "0" means start reading from the beginning of the stream.
	// "$" means only new messages — we use "0" to not miss anything on restart.
	err := redisClient.RDB.XGroupCreateMkStream(
		context.Background(), streamName, consumerGroup, "0",
	).Err()
	if err != nil {
		// BUSYGROUP = group already exists, which is fine.
		if !strings.HasPrefix(err.Error(), "BUSYGROUP") {
			return nil, fmt.Errorf("create consumer group: %w", err)
		}
	}

	log.Printf("[worker-pool] consumer group '%s' ready on stream '%s'", consumerGroup, streamName)
	return wp, nil
}

// Start launches all worker goroutines.
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workerCount; i++ {
		consumerName := fmt.Sprintf("%s-%d", wp.namePrefix, i)
		go wp.runWorker(consumerName)
	}
	log.Printf("[worker-pool] started %d workers", wp.workerCount)
}

// runWorker is the main loop for a single consumer.
// It continuously reads from the stream, processes each message, and ACKs it.
func (wp *WorkerPool) runWorker(consumerName string) {
	log.Printf("[worker:%s] started", consumerName)

	for {
		select {
		case <-wp.stopCh:
			log.Printf("[worker:%s] shutting down", consumerName)
			return
		default:
		}

		// XREADGROUP: read new messages (">") assigned to this consumer.
		// Block for up to 2 seconds waiting for new messages.
		streams, err := wp.redis.RDB.XReadGroup(context.Background(), &goredis.XReadGroupArgs{
			Group:    wp.consumerGroup,
			Consumer: consumerName,
			Streams:  []string{wp.streamName, ">"},
			Count:    10,              // batch size per read
			Block:    2 * time.Second, // block timeout — prevents busy-looping
		}).Result()

		if err != nil {
			// Timeout (no new messages) — just loop back.
			if err == goredis.Nil {
				continue
			}
			log.Printf("[worker:%s] XREADGROUP error: %v", consumerName, err)
			time.Sleep(500 * time.Millisecond) // backoff on error
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				wp.processMessage(consumerName, msg)
			}
		}
	}
}

// processMessage handles a single stream message: parse → debounce → ACK.
func (wp *WorkerPool) processMessage(consumerName string, msg goredis.XMessage) {
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		log.Printf("[worker:%s] msg %s: missing payload field", consumerName, msg.ID)
		// ACK it anyway to avoid reprocessing a malformed message forever.
		wp.ack(msg.ID)
		return
	}

	signal, err := ParseSignalFromPayload(payload)
	if err != nil {
		log.Printf("[worker:%s] msg %s: parse error: %v", consumerName, msg.ID, err)
		wp.ack(msg.ID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := wp.debouncer.ProcessSignal(ctx, signal); err != nil {
		log.Printf("[worker:%s] msg %s: process error: %v", consumerName, msg.ID, err)
		// Don't ACK — the message will be retried via pending entries list (PEL).
		return
	}

	wp.ack(msg.ID)
}

// ack acknowledges a message so it won't be re-delivered.
func (wp *WorkerPool) ack(msgID string) {
	err := wp.redis.RDB.XAck(context.Background(), wp.streamName, wp.consumerGroup, msgID).Err()
	if err != nil {
		log.Printf("[worker-pool] XACK error for %s: %v", msgID, err)
	}
}

// Stop signals all workers to shut down.
func (wp *WorkerPool) Stop() {
	close(wp.stopCh)
}

// GetPendingCount returns the number of unacknowledged messages in the consumer group.
// Used by the health endpoint to report consumer lag.
func (wp *WorkerPool) GetPendingCount(ctx context.Context) (int64, error) {
	info, err := wp.redis.RDB.XPending(ctx, wp.streamName, wp.consumerGroup).Result()
	if err != nil {
		return 0, err
	}
	return info.Count, nil
}
