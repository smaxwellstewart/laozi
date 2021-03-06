// Package laozi is an archiver of events.
// stores events to s3 partitioned however you want. imagine AWS firehose service but with a
// configurable partition method.
package laozi

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Laozi is an archiver responsible for receiving events and archiving them to
// a safe, reliable storage. Currently it achieves this by implementing s3 storage.
// It is named after one of the most famous archivists in the world, https://en.wikipedia.org/wiki/Laozi
type Laozi interface {
	Log([]byte)
	Close()
}

type laozi struct {
	sync.RWMutex
	EventChan  chan []byte
	routingMap map[string]Logger
	*Config
}

// Config is a struct used to configure Laozi to your implementation.
type Config struct {
	LoggerFactory    LoggerFactory
	LoggerTimeout    time.Duration
	PartitionKeyFunc func([]byte) (string, error)
	EventChannelSize int
}

func (c Config) valid() {
	if c.LoggerTimeout == time.Duration(0) {
		panic("LoggerTimeout must not be zero")
	}
	if c.PartitionKeyFunc == nil {
		panic("PartitionKeyFunc must be implemented")
	}
}

// NewLaozi creates a new router and start the logger monitoring
func NewLaozi(c *Config) Laozi {
	r := &laozi{
		EventChan:  make(chan []byte, c.EventChannelSize),
		routingMap: map[string]Logger{},
		Config:     c,
	}

	r.Config.valid()

	go r.monitorLoggers()
	go r.route()

	return r
}

// Log is designed as a non-blocking function for
// clients to use in a "fire and forget" manner
func (r *laozi) Log(e []byte) {
	r.EventChan <- e
}

// Close must be called whenever process terminates.
// This ensure all loggers have flushed their state.
func (r *laozi) Close() {
	for key, l := range r.routingMap {
		err := l.Close()
		if err != nil {
			fmt.Printf(" [laozi] Error! Could not close logger (possible data loss): %s\n", key)
		}
	}
}

// route listens to the EventChan for events and routes them to their according logger
// using the implemented partition key function.
func (r *laozi) route() {
	for e := range r.EventChan {

		key, err := r.PartitionKeyFunc(e)
		if err != nil {
			continue
		}

		// TODO: We need a way to test this
		r.Lock()
		l, found := r.routingMap[key]
		if !found {
			l = r.LoggerFactory.NewLogger(key)
			r.routingMap[key] = l

		}
		r.Unlock()
		l.Log(e)
	}
}

// monitorLoggers will periodically check the internal map and delete stale loggers.
func (r *laozi) monitorLoggers() {
	for _ = range time.Tick(r.LoggerTimeout / 2) {
		r.Lock()
		for key, l := range r.routingMap {
			if time.Since(l.LastActive()) >= r.LoggerTimeout {
				log.Printf("- [laozi] Logger timeout: %s\n", key)
				l.Close()
				delete(r.routingMap, key)
			}
		}
		r.Unlock()
	}
}
