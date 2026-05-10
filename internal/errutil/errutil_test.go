package errutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Nivl/config/internal/errutil"

	"github.com/stretchr/testify/assert"
)

type closer struct {
	actualClose func() error
}

func (c closer) Close() error {
	return c.actualClose()
}

// ctxCloser mirrors closer but threads a context.Context into the
// closure so the *Ctx variants of the helpers can be tested with
// the same shape of fixture.
type ctxCloser struct {
	actualClose func(context.Context) error
}

func (c ctxCloser) Close(ctx context.Context) error {
	return c.actualClose(ctx)
}

func TestRunAndSetError(t *testing.T) {
	t.Parallel()

	t.Run("Should call close and set the error", func(t *testing.T) {
		t.Parallel()

		errToReturn := errors.New("expected error")
		expectedErrMsg := "wrapper: " + errToReturn.Error()

		closed := false
		var err error
		c := closer{
			actualClose: func() error {
				closed = true
				return errToReturn
			},
		}

		errutil.RunAndSetError(c.Close, &err, "wrapper")
		assert.True(t, closed, "Close() should have been called")
		assert.Equal(t, expectedErrMsg, err.Error())
	})

	t.Run("Should call close and NOT set the error", func(t *testing.T) {
		t.Parallel()

		errToReturn := errors.New("expected error")

		closed := false
		err := errToReturn
		c := closer{
			actualClose: func() error {
				closed = true
				return errors.New("unexpected error")
			},
		}

		errutil.RunAndSetError(c.Close, &err, "wrapper")
		assert.True(t, closed, "Close() should have been called")
		assert.Equal(t, errToReturn, err)
	})

	t.Run("Should call close and set the error with a wrapped message", func(t *testing.T) {
		t.Parallel()

		errToReturn := errors.New("expected error")
		expectedErrMsg := "wrapper: " + errToReturn.Error()

		closed := false
		var err error
		c := closer{
			actualClose: func() error {
				closed = true
				return errToReturn
			},
		}

		errutil.RunAndSetError(c.Close, &err, "wrapper")
		assert.True(t, closed, "Close() should have been called")
		assert.Equal(t, expectedErrMsg, err.Error())
	})

	t.Run("Should do nothing", func(t *testing.T) {
		t.Parallel()

		closed := false
		var err error
		c := closer{
			actualClose: func() error {
				closed = true
				return nil
			},
		}

		errutil.RunAndSetError(c.Close, &err, "shouldn't show this")
		assert.True(t, closed, "Close() should have been called")
		assert.NoError(t, err)
	})
}

// TestRunAndSetErrorCtx — same four branches as TestRunAndSetError
// but for the context-aware variant. The ctx is also asserted to
// reach the callee so future refactors can't silently drop it.
func TestRunAndSetErrorCtx(t *testing.T) {
	t.Parallel()

	type ctxKey struct{}
	parentCtx := context.WithValue(context.Background(), ctxKey{}, "marker")

	t.Run("Should call close and set the error", func(t *testing.T) {
		t.Parallel()

		errToReturn := errors.New("expected error")
		var err error
		var receivedMarker any
		c := ctxCloser{
			actualClose: func(ctx context.Context) error {
				receivedMarker = ctx.Value(ctxKey{})
				return errToReturn
			},
		}

		errutil.RunAndSetErrorCtx(parentCtx, c.Close, &err, "wrapper")
		assert.Equal(t, "marker", receivedMarker, "context must be passed through verbatim")
		assert.Equal(t, "wrapper: expected error", err.Error())
	})

	t.Run("Should call close and NOT overwrite an existing error", func(t *testing.T) {
		t.Parallel()

		preExisting := errors.New("pre-existing")
		err := preExisting
		c := ctxCloser{
			actualClose: func(context.Context) error {
				return errors.New("ignored")
			},
		}

		errutil.RunAndSetErrorCtx(parentCtx, c.Close, &err, "wrapper")
		assert.Equal(t, preExisting, err, "existing non-nil err must not be replaced")
	})

	t.Run("Should not wrap when msg is empty", func(t *testing.T) {
		t.Parallel()

		errToReturn := errors.New("naked")
		var err error
		c := ctxCloser{actualClose: func(context.Context) error { return errToReturn }}

		errutil.RunAndSetErrorCtx(parentCtx, c.Close, &err, "")
		assert.Equal(t, errToReturn, err, "empty msg passes the error through unwrapped")
	})

	t.Run("Should do nothing when callee returns nil", func(t *testing.T) {
		t.Parallel()

		var err error
		called := false
		c := ctxCloser{actualClose: func(context.Context) error { called = true; return nil }}

		errutil.RunAndSetErrorCtx(parentCtx, c.Close, &err, "wrapper")
		assert.True(t, called, "callee must still be invoked")
		assert.NoError(t, err)
	})
}

// TestRunOnErr — the "only set on existing error" variant: when the
// to-check error is nil the callee is NOT invoked; when it's non-nil
// the callee runs and any error it returns wraps via msg. Since
// RunOnErr takes toCheck by value (not pointer), the caller's err
// is not mutated — the helper is a logging/cleanup-aid, not an
// error-aggregator.
func TestRunOnErr(t *testing.T) {
	t.Parallel()

	t.Run("Callee is skipped when toCheck is nil", func(t *testing.T) {
		t.Parallel()

		called := false
		errutil.RunOnErr(func() error { called = true; return nil }, nil, "wrapper")
		assert.False(t, called, "RunOnErr must not invoke callee when toCheck is nil")
	})

	t.Run("Callee runs when toCheck is non-nil", func(t *testing.T) {
		t.Parallel()

		preExisting := errors.New("pre-existing")
		called := false
		errutil.RunOnErr(func() error { called = true; return nil }, preExisting, "wrapper")
		assert.True(t, called, "RunOnErr must invoke callee when toCheck is non-nil")
	})
}

// TestRunOnErrWithCtx — ctx-aware version of TestRunOnErr; same
// gating contract (only fires when toCheck is non-nil), but the
// callee receives the ctx.
func TestRunOnErrWithCtx(t *testing.T) {
	t.Parallel()

	type ctxKey struct{}
	parentCtx := context.WithValue(context.Background(), ctxKey{}, "marker")

	t.Run("Callee is skipped when toCheck is nil", func(t *testing.T) {
		t.Parallel()

		called := false
		errutil.RunOnErrWithCtx(parentCtx,
			func(context.Context) error { called = true; return nil },
			nil, "wrapper")
		assert.False(t, called)
	})

	t.Run("Callee runs with the supplied ctx when toCheck is non-nil", func(t *testing.T) {
		t.Parallel()

		var receivedMarker any
		errutil.RunOnErrWithCtx(parentCtx,
			func(ctx context.Context) error {
				receivedMarker = ctx.Value(ctxKey{})
				return nil
			},
			errors.New("pre-existing"), "wrapper")
		assert.Equal(t, "marker", receivedMarker, "context must be passed through")
	})
}
