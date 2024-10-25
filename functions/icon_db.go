package functions

import (
	"database/sql"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type MyDBInterface struct {
	db *gorm.DB
}

func NewDBInterface(dsn string, opts ...gorm.Option) *MyDBInterface {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  "host=localhost user=" + os.Getenv("PSQL_USER") + " password=" + os.Getenv("PSQL_PASSWD") + " dbname=" + os.Getenv("PSQL_DBNAME") + " port=5432 sslmode=disable TimeZone=Asia/Shanghai", // data source name, refer https://github.com/jackc/pgx
		PreferSimpleProtocol: true,                                                                                                                                                                                    // disables implicit prepared statement usage. By default pgx automatically uses the extended protocol
	}), &gorm.Config{})

	if err != nil {
		panic("failed to connect database")
	}
	return &MyDBInterface{db}
}

func (i *MyDBInterface) Begin(opts ...*sql.TxOptions) *gorm.DB {
	return i.db.Begin(opts...)
}
func (i *MyDBInterface) AutoMigrate(dst ...any) error {
	return i.db.AutoMigrate(dst...)
}
func (i *MyDBInterface) Raw(sql string, values ...any) *gorm.DB {
	return i.db.Raw(sql, values...)
}

func Create(tx *gorm.DB, o any) error {
	tx.Create(o)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
func Read(tx *gorm.DB, o any) error {
	tx.Where(o).First(o)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
func Update(tx *gorm.DB, o any) error {
	tx.Save(o)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
func Delete(tx *gorm.DB, o any) error {
	tx.Delete(o)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}
