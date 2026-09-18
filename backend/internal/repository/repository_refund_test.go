package repository

import (
	"errors"
	"fmt"
	"testing"
)

func TestConflictSentinel(t *testing.T) {
	if !errors.Is(fmt.Errorf("create refund application: %w", ErrConflict), ErrConflict) {
		t.Fatal("wrapped ErrConflict should be detectable with errors.Is")
	}
}

func TestNotFoundSentinel(t *testing.T) {
	if !errors.Is(fmt.Errorf("find donation by id: %w", ErrNotFound), ErrNotFound) {
		t.Fatal("wrapped ErrNotFound should be detectable with errors.Is")
	}
}
