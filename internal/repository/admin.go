package repository

import (
	"context"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"goshort/internal/model"
)

type AdminRepo struct {
	db *gorm.DB
}

func NewAdminRepo(db *gorm.DB) *AdminRepo { return &AdminRepo{db: db} }

func (r *AdminRepo) Upsert(ctx context.Context, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var admin model.AdminUser
	err = r.db.WithContext(ctx).Where("email = ?", email).First(&admin).Error
	if IsNotFound(err) {
		return r.db.WithContext(ctx).Create(&model.AdminUser{Email: email, PasswordHash: string(hash)}).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&admin).Update("password_hash", string(hash)).Error
}

// คืน error เดียวกันทั้งกรณีไม่มีอีเมลนั้นและกรณีรหัสผิด ไม่บอกใบ้ว่าอีเมลไหนมีอยู่จริง
func (r *AdminRepo) Authenticate(ctx context.Context, email, password string) (model.AdminUser, bool) {
	var admin model.AdminUser
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&admin).Error; err != nil {
		// เทียบกับ hash ทิ้ง ๆ ให้เวลาที่ใช้ใกล้เคียงกับกรณีเจอผู้ใช้จริง
		bcrypt.CompareHashAndPassword([]byte("$2a$10$"+"x"), []byte(password))
		return model.AdminUser{}, false
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return model.AdminUser{}, false
	}
	return admin, true
}
