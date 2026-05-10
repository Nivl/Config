// Package errutil contains methods to simplify working with error
package errutil

import (
	"context"
	"fmt"
	"log"
)

// RunAndSetError calls the provided method and sets the error to err
// if err is nil
func RunAndSetError(f func() error, err *error, msg string) { //nolint: gocritic // the pointer of pointer is on purpose so we can change the value if it's nil
	if e := f(); e != nil {
		switch *err { //nolint: gocritic // no need to use errors.Is here since we're only checking for "is nil or not"
		case nil:
			wrappedErr := e
			if msg != "" {
				wrappedErr = fmt.Errorf("%s: %w", msg, e)
			}
			*err = wrappedErr
		default:
			// TODO(melvin):use the app logger instead
			log.Println("a check failed:", e)
		}
	}
}

// RunAndSetErrorCtx calls the provided method and sets the error to err
// if err is nil
func RunAndSetErrorCtx(ctx context.Context, f func(context.Context) error, err *error, msg string) { //nolint: gocritic // the pointer of pointer is on purpose so we can change the value if it's nil
	if e := f(ctx); e != nil {
		switch *err { //nolint: gocritic // no need to use errors.Is here since we're only checking for "is nil or not"
		case nil:
			wrappedErr := e
			if msg != "" {
				wrappedErr = fmt.Errorf("%s: %w", msg, e)
			}
			*err = wrappedErr
		default:
			// TODO(melvin):use the app logger instead
			log.Println("a check failed:", e)
		}
	}
}

// RunOnErrWithCtx calls the provided method if the provided error is not nil.
// If the provided method returns an error, it will be set to
// the provided error.
func RunOnErrWithCtx(ctx context.Context, toRun func(context.Context) error, toCheck error, msgIfFuncFailed string) {
	if toCheck != nil {
		RunAndSetErrorCtx(ctx, toRun, &toCheck, msgIfFuncFailed)
	}
}

// RunOnErr calls the provided method if the provided error is not nil.
// If the provided method returns an error, it will be set to
// the provided error.
func RunOnErr(toRun func() error, toCheck error, msgIfFuncFailed string) {
	if toCheck != nil {
		RunAndSetError(toRun, &toCheck, msgIfFuncFailed)
	}
}
