// Package eventstream implements a convenience client for the Events method
// of the Tendermint RPC service, allowing clients to observe a resumable
// stream of events matching a query.
package eventstream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/rpc/coretypes"
)

// Client is the subset of the RPC client interface consumed by Stream.
type Client interface {
	Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error)
}

// ErrStopRunning is returned by a Run callback to signal that no more events
// are wanted and that Run should return.
var ErrStopRunning = errors.New("stop accepting events")

// A Stream cpatures the state of a streaming event subscription.
type Stream struct {
	filter     *coretypes.EventFilter // the query being streamed
	batchSize  int                    // request batch size
	newestSeen string                 // from the latest item matching our query
	waitTime   time.Duration          // the long-polling interval
	client     Client
}

// New constructs a new stream for the given query and options.
// If opts == nil, the stream uses default values as described by
// StreamOptions.  This function will panic if cli == nil.
func New(cli Client, query string, opts *StreamOptions) *Stream {
	if cli == nil {
		panic("eventstream: nil client")
	}
	return &Stream{
		filter:     &coretypes.EventFilter{Query: query},
		batchSize:  opts.batchSize(),
		newestSeen: opts.resumeFrom(),
		waitTime:   opts.waitTime(),
		client:     cli,
	}
}

// Run polls the service for events matching the query, and calls accept for
// each such event. Run handles pagination transparently, and delivers events
// to accept in order of publication.
//
// Run continues until ctx ends or accept reports an error.  If accept returns
// ErrStopRunning, Run returns nil; otherwise Run returns the error reported by
// accept or ctx.  Run also returns an error if the server reports an error
// from the Events method.
//
// If the stream falls behind the event log on the server, Run will stop and
// report an error of concrete type *MissedItemsError.  Call Reset to reset the
// stream to the head of the log, and call Run again to resume.
func (s *Stream) Run(ctx context.Context, accept func(*coretypes.EventItem) error) error {
	for {
		items, err := s.fetchPages(ctx)
		if err != nil {
			return err
		}

		// Deliver events from the current batch to the receiver.  We visit the
		// batch in reverse order so the receiver sees them in forward order.
		for i := len(items) - 1; i >= 0; i-- {
			if err := ctx.Err(); err != nil {
				return err
			}

			itm := items[i]
			err := accept(itm)
			if itm.Cursor > s.newestSeen {
				s.newestSeen = itm.Cursor // update the latest delivered
			}
			if errors.Is(err, ErrStopRunning) {
				return nil
			} else if err != nil {
				return err
			}
		}
	}
}

// Reset updates the stream's current cursor position to the head of the log.
// This method may safely be called only when Run is not executing.
func (s *Stream) Reset() { s.newestSeen = "" }

// fetchPages fetches the next batch of matching results. If there are multiple
// pages, all the matching pages are retrieved. An error is reported if the
// current scan position falls out of the event log window.
func (s *Stream) fetchPages(ctx context.Context) ([]*coretypes.EventItem, error) {
	var pageCursor string // if non-empty, page through items before this
	var items []*coretypes.EventItem

	// Fetch the next paginated batch of matching responses.
	for {
		rsp, err := s.client.Events(ctx, &coretypes.RequestEvents{
			Filter:   s.filter,
			MaxItems: s.batchSize,
			After:    s.newestSeen,
			Before:   pageCursor,
			WaitTime: s.waitTime,
		})
		if err != nil {
			return nil, err
		}

		// If the oldest item in the log is newer than our most recent item,
		// it means we might have missed some events matching our query.
		if s.newestSeen != "" && s.newestSeen < rsp.Oldest {
			return nil, &MissedItemsError{
				Query:         s.filter.Query,
				NewestSeen:    s.newestSeen,
				OldestPresent: rsp.Oldest,
			}
		}
		items = append(items, rsp.Items...)

		if rsp.More {
			// There are more results matching this request, leave the baseline
			// where it is and set the page cursor so that subsequent requests
			// will get the next chunk.
			pageCursor = items[len(items)-1].Cursor
		} else if len(items) != 0 {
			// We got everything matching so far.
			return items, nil
		}
	}
}

// StreamOptions are optional settings for a Stream value. A nil *StreamOptions
// is ready for use and provides default values as described.
type StreamOptions struct {
	// How many items to request per call to the service.  The stream may pin
	// this value to a minimum default batch size.
	BatchSize int

	// If set, resume streaming from this cursor. Typically this is set to the
	// cursor of the most recently-received matching value. If empty, streaming
	// begins at the head of the log (the default).
	ResumeFrom string

	// Specifies the long poll interval. The stream may pin this value to a
	// minimum default poll interval.
	WaitTime time.Duration
}

func (o *StreamOptions) batchSize() int {
	const minBatchSize = 16
	if o == nil || o.BatchSize < minBatchSize {
		return minBatchSize
	}
	return o.BatchSize
}

func (o *StreamOptions) resumeFrom() string {
	if o == nil {
		return ""
	}
	return o.ResumeFrom
}

func (o *StreamOptions) waitTime() time.Duration {
	const minWaitTime = 5 * time.Second
	if o == nil || o.WaitTime < minWaitTime {
		return minWaitTime
	}
	return o.WaitTime
}

// MissedItemsError is an error that indicates the stream missed (lost) some
// number of events matching the specified query.
type MissedItemsError struct {
	// The cursor of the newest matching item the stream has observed.
	NewestSeen string

	// The oldest cursor in the log at the point the miss was detected.
	// Any matching events between NewestSeen and OldestPresent are lost.
	OldestPresent string

	// The active query.
	Query string
}

// Error satisfies the error interface.
func (e *MissedItemsError) Error() string {
	return fmt.Sprintf("missed events matching %q between %q and %q", e.Query, e.NewestSeen, e.OldestPresent)
}
