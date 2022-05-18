package helper

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/acorn-io/baaah/pkg/meta"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	WatchTimeoutSeconds int64 = 120
)

type WatchFunc func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) (watch.Interface, error)
type watchFunc func() (watch.Interface, error)

func doWatch[T meta.Object](t *testing.T, watchFunc watchFunc, cb func(obj T) bool) bool {
	ctx := GetCTX(t)
	result, err := watchFunc()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		result.Stop()
		for range result.ResultChan() {
		}
	}()

	for {
		select {
		case <-ctx.Done():
			t.Fatal(fmt.Errorf("timeout waiting condition: %w", ctx.Err()))
		case event, open := <-result.ResultChan():
			if !open {
				return false
			}
			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				done := cb(event.Object.(T))
				if done {
					if apierrors.IsConflict(err) {
						// if we got a conflict, return a false (not done) and nil for error
						return false
					}
					if err != nil {
						t.Fatal(err)
					}
					return true
				}
			}
		}
	}
}

func retryWatch[T meta.Object](t *testing.T, watchFunc watchFunc, cb func(obj T) bool) {
	for {
		if done := doWatch(t, watchFunc, cb); done {
			return
		}
	}
}

func Wait[T meta.Object](t *testing.T, watchFunc WatchFunc, list meta.ObjectList, cb func(obj T) bool) T {
	var last T
	retryWatch(t, func() (watch.Interface, error) {
		ctx := GetCTX(t)
		return watchFunc(ctx, list)
	}, func(obj T) bool {
		last = obj
		return cb(obj)
	})
	return last
}

func WaitForObject[T meta.Object](t *testing.T, watchFunc WatchFunc, list meta.ObjectList, obj T, cb func(obj T) bool) T {
	if done := cb(obj); done {
		return obj
	}

	var last T
	retryWatch(t, func() (watch.Interface, error) {
		ctx := GetCTX(t)
		return watchFunc(ctx, list, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", obj.GetName()),
			Namespace:     obj.GetNamespace(),
		})
	}, func(obj T) bool {
		last = obj
		return cb(obj)
	})
	return last
}

func EnsureDoesNotExist(ctx context.Context, getter func() (runtime.Object, error)) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(WatchTimeoutSeconds)*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deletion: %w", ctx.Err())
		case <-ticker.C:
			_, err := getter()
			if apierrors.IsNotFound(err) {
				return nil
			} else if err != nil {
				return err
			}
		}
	}
}
