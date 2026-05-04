package redis

import (
	"context"
	"encoding/json"
	"log"

	goredis "github.com/redis/go-redis/v9"
	"github.com/tf-vishal/zeotap-ims/internal/models"
)

// StreamBuffer provides non-blocking signal buffering via a Go channel
// and a background goroutine that performs Redis XADD operations.
type StreamBuffer struct {
	client     *Client
	streamName string
	ch         chan *models.Signal
}

// bufferSize controls the internal channel capacity.
// A large buffer absorbs short bursts without back-pressure on the HTTP handler.
const bufferSize = 50_000

// NewStreamBuffer creates the buffer and starts background flushing goroutines.
// numWorkers controls parallelism for Redis writes.
func NewStreamBuffer(client *Client, streamName string, numWorkers int) *StreamBuffer {
	sb := &StreamBuffer{
		client:     client,
		streamName: streamName,
		ch:         make(chan *models.Signal, bufferSize),
	}

	for i := 0; i < numWorkers; i++ {
		go sb.worker(i)
	}

	log.Printf("[stream] buffer started — capacity=%d workers=%d stream=%s",
		bufferSize, numWorkers, streamName)
	return sb
}

// Push enqueues a signal for async writing to Redis.
// Returns false if the channel is full (back-pressure signal to caller).
func (sb *StreamBuffer) Push(signal *models.Signal) bool {
	select {
	case sb.ch <- signal:
		return true
	default:
		// Channel full — drop signal rather than block the HTTP handler.
		return false
	}
}

// worker reads from the channel and writes to the Redis stream.
func (sb *StreamBuffer) worker(id int) {
	for signal := range sb.ch {
		// Serialize the signal to a flat map for XADD.
		data, err := json.Marshal(signal)
		if err != nil {
			log.Printf("[stream] worker-%d marshal error: %v", id, err)
			continue
		}

		err = sb.client.RDB.XAdd(context.Background(), &goredis.XAddArgs{
			Stream: sb.streamName,
			ID:     "*", // let Redis auto-generate the ID
			Values: map[string]interface{}{
				"payload": string(data),
			},
		}).Err()

		if err != nil {
			log.Printf("[stream] worker-%d XADD error: %v", id, err)
		}
	}
}
