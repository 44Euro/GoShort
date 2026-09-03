package repository

import (
	"context"

	"gorm.io/gorm"

	"goshort/internal/model"
	"goshort/internal/worker"
)

type ClickStore struct {
	db *gorm.DB
}

func NewClickStore(db *gorm.DB) *ClickStore { return &ClickStore{db: db} }

func (s *ClickStore) WriteBatch(ctx context.Context, events []worker.Event) error {
	rows := make([]model.ClickEvent, 0, len(events))
	counts := map[uint]int64{}

	for _, e := range events {
		rows = append(rows, model.ClickEvent{
			LinkID:    e.LinkID,
			IPHash:    e.IPHash,
			UserAgent: e.UserAgent,
			Referrer:  e.Referrer,
			CreatedAt: e.At,
		})
		counts[e.LinkID]++
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(&rows, len(rows)).Error; err != nil {
			return err
		}
		// อ่านค่าเดิมมาบวกใน Go ไม่ได้ หลาย worker แก้ link เดียวกันพร้อมกันแล้วยอดหาย
		// ต้องให้ Postgres บวกให้เอง และรวมยอดในหนึ่ง batch ก่อนเพื่อยิง UPDATE ครั้งเดียวต่อ link
		for linkID, n := range counts {
			err := tx.Model(&model.Link{}).
				Where("id = ?", linkID).
				UpdateColumn("click_count", gorm.Expr("click_count + ?", n)).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}
