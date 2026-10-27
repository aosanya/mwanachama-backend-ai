package ai

import (
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func Provision(db *gorm.DB, s *spec.Spec) error {
	return spec.Migrate(db, s)
}
