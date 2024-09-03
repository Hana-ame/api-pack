package functions

import (
	"fmt"

	"gorm.io/gorm"
)

func init() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("Recovered from:", err)
		}
	}()

	iconDB = NewDBInterface("ginpack/icons.db", &gorm.Config{})
	iconDB.AutoMigrate(new(myKV))
}
