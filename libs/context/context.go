// Package context provides helpers to move between APIs that use
// contexts and APIs that use singal channels.
package context

import "context"

// WithSignalChannel wraps an existing context and a singal channel,
// and will cancel the returned context (and it's children) when the
// signal channel closes.
func WithSignalChannel(ctx context.Context, done <-chan struct{}) (context.Context, context.CancelFunc) {
	return WithSignalChanelClose(ctx, done, func() {})
}

// TODO is a shim for situations when callers do not have an existing
// context, and wish to convert a close channel into a context.
func TODO(done <-chan struct{}) (context.Context, context.CancelFunc) {
	return WithSignalChannel(context.TODO(), done)
}

// WithSignalChanelClose wraps an existing channel and a signal
// channel, canceling the returned context when the context is
// canceled or signal channel is closed. In either case, the afterClose
// function is always called after the context is closed or the done
// channel signals.
func WithSignalChanelClose(
	ctx context.Context,
	done <-chan struct{},
	afterClose func()) (context.Context, context.CancelFunc) {

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	go func() {
		defer afterClose()
		select {
		case <-done:
			// if the signal channel fires, cancel the
			// context.
			cancel()
		case <-ctx.Done():
			// release the thread when if the outer
			// context is canceled.
		}
	}()

	return ctx, cancel
}
