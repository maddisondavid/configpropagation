package core

import (
	"context"
	"errors"
	"net"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ErrorCategory describes the class of an error encountered while reconciling.
type ErrorCategory string

const (
	// ErrorCategoryNone indicates no error.
	ErrorCategoryNone ErrorCategory = ""
	// ErrorCategoryRBAC indicates insufficient permissions (Forbidden/Unauthorized).
	ErrorCategoryRBAC ErrorCategory = "rbac"
	// ErrorCategoryTransient indicates a retryable/transient failure.
	ErrorCategoryTransient ErrorCategory = "transient"
	// ErrorCategoryPermanent indicates a non-retryable failure unrelated to RBAC.
	ErrorCategoryPermanent ErrorCategory = "permanent"
)

// ClassifiedError wraps an error with its detected category.
type ClassifiedError struct {
	Err      error
	Category ErrorCategory
}

func (e *ClassifiedError) Error() string { return e.Err.Error() }

func (e *ClassifiedError) Unwrap() error { return e.Err }

// ClassifyError inspects an error and returns the appropriate category.
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryNone
	}
	// Walk the error chain to find a concrete classification.
	for current := err; current != nil; current = errors.Unwrap(current) {
		switch {
		case apierrors.IsForbidden(current) || apierrors.IsUnauthorized(current):
			return ErrorCategoryRBAC
		case apierrors.IsTooManyRequests(current), apierrors.IsTimeout(current), apierrors.IsServerTimeout(current):
			return ErrorCategoryTransient
		}
		// Handle context cancellations and deadlines as transient issues.
		if errors.Is(current, context.DeadlineExceeded) || errors.Is(current, context.Canceled) {
			return ErrorCategoryTransient
		}
		// Net errors can expose retry semantics via the Temporary method.
		if ne, ok := current.(net.Error); ok {
			if ne.Timeout() || ne.Temporary() {
				return ErrorCategoryTransient
			}
		}
	}
	return ErrorCategoryPermanent
}
