package service

import (
	"strings"
	"testing"

	"github.com/givetrack/givetrack/internal/constants"
)

func TestValidateApplyInput(t *testing.T) {
	tests := []struct {
		name    string
		in      ApplyInput
		wantErr string
	}{
		{name: "empty reason", in: ApplyInput{Reason: ""}, wantErr: "reason"},
		{name: "blank reason", in: ApplyInput{Reason: "   "}, wantErr: "reason"},
		{name: "valid", in: ApplyInput{Reason: "误捐，申请全额退款"}, wantErr: ""},
		{name: "too long", in: ApplyInput{Reason: strings.Repeat("款", 501)}, wantErr: "too long"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApplyInput(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateReviewInput(t *testing.T) {
	tests := []struct {
		name    string
		in      ReviewInput
		wantErr bool
	}{
		{name: "approve", in: ReviewInput{Status: constants.RefundApproved}},
		{name: "reject", in: ReviewInput{Status: constants.RefundRejected, Note: "材料不符"}},
		{name: "invalid status", in: ReviewInput{Status: "pending"}, wantErr: true},
		{name: "empty status", in: ReviewInput{}, wantErr: true},
		{name: "note too long", in: ReviewInput{Status: constants.RefundApproved, Note: strings.Repeat("x", 256)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReviewInput(tt.in)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsDuplicateKeyErr(t *testing.T) {
	if isDuplicateKeyErr(nil) {
		t.Fatal("nil should not be duplicate")
	}
	if !isDuplicateKeyErr(&testErr{"Error 1062: Duplicate entry '5' for key 'uidx_donation'"}) {
		t.Fatal("expected duplicate key detection")
	}
	if isDuplicateKeyErr(&testErr{"some other error"}) {
		t.Fatal("unexpected duplicate detection")
	}
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
