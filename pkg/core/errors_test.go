package core

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type transientNetError struct{}

func (transientNetError) Error() string   { return "temporary" }
func (transientNetError) Timeout() bool   { return true }
func (transientNetError) Temporary() bool { return true }

func TestClassifyError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrorCategory
	}{{
		name: "nil",
		err:  nil,
		want: ErrorCategoryNone,
	}, {
		name: "forbidden",
		err:  apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "configmaps"}, "cm", errors.New("denied")),
		want: ErrorCategoryRBAC,
	}, {
		name: "unauthorized",
		err:  apierrors.NewUnauthorized("nope"),
		want: ErrorCategoryRBAC,
	}, {
		name: "timeout",
		err:  apierrors.NewTimeoutError("slow", 1),
		want: ErrorCategoryTransient,
	}, {
		name: "too many requests",
		err:  apierrors.NewTooManyRequests("back off", 0),
		want: ErrorCategoryTransient,
	}, {
		name: "context deadline",
		err:  context.DeadlineExceeded,
		want: ErrorCategoryTransient,
	}, {
		name: "net temporary",
		err:  transientNetError{},
		want: ErrorCategoryTransient,
	}, {
		name: "wrapped",
		err:  fmtErrorWrapper(apierrors.NewForbidden(schema.GroupResource{Group: "", Resource: "configmaps"}, "cm", errors.New("denied"))),
		want: ErrorCategoryRBAC,
	}, {
		name: "permanent",
		err:  errors.New("boom"),
		want: ErrorCategoryPermanent,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyError(tc.err); got != tc.want {
				t.Fatalf("expected %s got %s", tc.want, got)
			}
		})
	}
}

// fmtErrorWrapper wraps an error with fmt.Errorf to ensure unwrap works.
func fmtErrorWrapper(err error) error { return fmt.Errorf("wrapped: %w", err) }
