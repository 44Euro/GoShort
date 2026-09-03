package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"goshort/internal/model"
)

// ตัดตัวที่ตาสับสนออก (0/O, 1/l/I) เพราะ code ถูกอ่านออกเสียงและพิมพ์ตามบ่อย
const codeAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"

const (
	codeLength   = 7
	codeAttempts = 5
)

var ErrCodeTaken = errors.New("short code already taken")

type LinkRepo struct {
	db *gorm.DB
}

func NewLinkRepo(db *gorm.DB) *LinkRepo { return &LinkRepo{db: db} }

func (r *LinkRepo) CreateWithAlias(ctx context.Context, alias, longURL string, expiresAt *time.Time) (model.Link, error) {
	link := model.Link{ShortCode: alias, LongURL: longURL, CustomAlias: true, ExpiresAt: expiresAt}
	if err := r.db.WithContext(ctx).Create(&link).Error; err != nil {
		if isDuplicate(err) {
			return model.Link{}, ErrCodeTaken
		}
		return model.Link{}, err
	}
	return link, nil
}

// ชนกันปล่อยให้ unique index จับแล้ว retry ห้ามเช็คว่าว่างก่อนแล้วค่อยเขียน
// เพราะระหว่างสองคำสั่งนั้นมีช่องให้ goroutine อื่นแทรก
func (r *LinkRepo) Create(ctx context.Context, longURL string, expiresAt *time.Time) (model.Link, error) {
	for i := 0; i < codeAttempts; i++ {
		code, err := generateCode()
		if err != nil {
			return model.Link{}, err
		}
		link := model.Link{ShortCode: code, LongURL: longURL, ExpiresAt: expiresAt}
		err = r.db.WithContext(ctx).Create(&link).Error
		if err == nil {
			return link, nil
		}
		if !isDuplicate(err) {
			return model.Link{}, err
		}
	}
	return model.Link{}, fmt.Errorf("could not find a free short code in %d attempts", codeAttempts)
}

func generateCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, codeLength)
	for i, b := range buf {
		out[i] = codeAlphabet[int(b)%len(codeAlphabet)]
	}
	return string(out), nil
}

func isDuplicate(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey)
}

func (r *LinkRepo) ByCode(ctx context.Context, code string) (model.Link, error) {
	var link model.Link
	err := r.db.WithContext(ctx).Where("short_code = ?", code).First(&link).Error
	return link, err
}

func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
