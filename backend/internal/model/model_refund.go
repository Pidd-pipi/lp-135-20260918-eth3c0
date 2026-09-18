package model

import "time"

// RefundApplication 捐赠退款申请。
// 同一笔捐赠只能有一条退款申请（DonationID 唯一索引），
// 管理员仅能对处于 pending 的申请审核一次。
type RefundApplication struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	DonationID   uint       `gorm:"index;not null" json:"donationId"`
	UserID       uint       `gorm:"index;not null" json:"userId"`
	ProjectID    uint       `gorm:"index;not null" json:"projectId"`
	Amount       float64    `gorm:"type:decimal(14,2);not null" json:"amount"`
	Reason       string     `gorm:"size:255;not null" json:"reason"`
	Status       string     `gorm:"size:20;index;not null;default:pending" json:"status"`
	ReviewReason string     `gorm:"size:255" json:"reviewReason"`
	ReviewerID   uint       `gorm:"index" json:"reviewerId"`
	ReviewedAt   *time.Time `json:"reviewedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`

	Donation *Donation `gorm:"foreignKey:DonationID" json:"donation,omitempty"`
	Project  *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Reviewer *User     `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
}
