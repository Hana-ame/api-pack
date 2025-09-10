package pastejson

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	tools "github.com/Hana-ame/api-pack/Tools"
	"github.com/jackc/pgx/v5"
	"github.com/valyala/fastjson"
)

func TestPost(t *testing.T) {
	db, err := NewDBPool(nil, connStr)
	if err != nil {
		t.Error(err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"@metaData": map[string]interface{}{
			"tags": []any{"1", 2.2},
		},
		"key":  "value1",
		"key2": "value2",
	})
	for i := 0; i < 10; i++ {
		err = db.ExecuteInTransaction(context.Background(), pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {

			id := tools.NewTimeStamp()
			if err := AddJsonData(ctx, tx, id, body); err != nil {
				return err
			}
			var p fastjson.Parser
			if v, err := p.ParseBytes(body); err == nil {
				tags := v.GetArray("@metaData", "tags")
				for _, tag := range tags {
					if err := AddTag(ctx, tx, id, string(tag.GetStringBytes())); err != nil {
						// return err
						// 不需要退出
					}
				}
			}
			if v, err := p.ParseBytes(body); err == nil {
				previous := v.GetArray("@metaData", "previous")
				for _, previousID := range previous {
					if err := AddPrevious(ctx, tx, previousID.GetInt64(), id); err != nil {
						// return err
						// 不需要退出
					}
				}
			}

			return nil
		})
	}
}

func TestGetByTag(t *testing.T) {

	fmt.Println("v3")

	db, err := NewDBPool(nil, connStr)
	if err != nil {
		t.Error(err)
	}

	db.ExecuteInTransaction(context.Background(), pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
		r, e := GetRecordsByTag(ctx, tx, "22", 2, 115177874571264000, 0)
		s, _ := json.Marshal(r)
		fmt.Printf("%s", s)
		return e
	})
}
