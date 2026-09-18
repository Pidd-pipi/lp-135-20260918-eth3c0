package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
)

func TestValidateRefundApply(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		d       *model.Donation
		wantErr string
	}{
		{
			name:    "within window success",
			d:       &model.Donation{PaymentStatus: constants.PaymentSuccess, CreatedAt: now.Add(-23 * time.Hour)},
			wantErr: "",
		},
		{
			name:    "exactly at boundary 24h allowed",
			d:       &model.Donation{PaymentStatus: constants.PaymentSuccess, CreatedAt: now.Add(-RefundApplyWindow)},
			wantErr: "",
		},
		{
			name:    "over 24 hours conflict",
			d:       &model.Donation{PaymentStatus: constants.PaymentSuccess, CreatedAt: now.Add(-25 * time.Hour)},
			wantErr: "24 小时",
		},
		{
			name:    "already refunded conflict",
			d:       &model.Donation{PaymentStatus: constants.PaymentRefunded, CreatedAt: now.Add(-time.Hour)},
			wantErr: "已退款",
		},
		{
			name:    "pending payment conflict",
			d:       &model.Donation{PaymentStatus: constants.PaymentPending, CreatedAt: now.Add(-time.Hour)},
			wantErr: "已退款",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRefundApply(tt.d, now)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !errors.Is(err, repository.ErrConflict) {
				t.Fatalf("expected ErrConflict, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected message containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestRefundApplyWindow(t *testing.T) {
	if RefundApplyWindow != 24*time.Hour {
		t.Fatalf("RefundApplyWindow = %v, want 24h", RefundApplyWindow)
	}
}
