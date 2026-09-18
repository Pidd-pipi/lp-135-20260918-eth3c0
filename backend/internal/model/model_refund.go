package model

import "time"

// RefundApplication 公益捐赠退款申请。
// 同一笔捐赠至多存在一条申请（donation_id 唯一索引），保证“同一笔只能有一个待审申请”。
type RefundApplication struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DonationID uint       `gorm:"uniqueIndex;not null" json:"donationId"`
	UserID     uint       `gorm:"index;not null" json:"userId"`
	Reason     string     `gorm:"type:text;not null" json:"reason"`
	Status     string     `gorm:"size:20;index;default:pending" json:"status"`
	ReviewerID uint       `gorm:"index;default:0" json:"reviewerId"`
	ReviewNote string     `gorm:"size:255" json:"reviewNote"`
	ReviewedAt *time.Time `json:"reviewedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	Donation   *Donation  `gorm:"foreignKey:DonationID" json:"donation,omitempty"`
	User       *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Reviewer   *User      `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
}
