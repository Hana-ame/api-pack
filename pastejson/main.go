package pastejson

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/Hana-ame/api-pack/Tools"
	tools "github.com/Hana-ame/api-pack/Tools"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/valyala/fastjson"
)

func GetJsonHandler(db *DBPool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(
			tools.NewSlice(strings.Split(c.Param("id"), ".")...).GetOrDefault(0, "0"), 10, 64)

		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}
		if id == 0 {
			tools.AbortWithError(c, http.StatusBadRequest, fmt.Errorf("id is invalid"))
			return
		}

		err = db.ExecuteInTransaction(context.Background(), pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
			data, err := GetRecordByID(ctx, tx, id)
			if err != nil {
				return err
			}
			c.Data(http.StatusOK, "application/json", data)
			return nil
		})

		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}
	}
}
func GetTagHandler(db *DBPool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tag := c.Param("tag")
		greaterThan := tools.Match(strconv.Atoi(c.Query("gt"))).GetOrDefault(0)
		lessThan := tools.Match(strconv.Atoi(c.Query("lt"))).GetOrDefault(0)
		limit := tools.Match(strconv.Atoi(c.Query("limit"))).GetOrDefault(10)

		err := db.ExecuteInTransaction(context.Background(), pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
			data, err := GetRecordsByTag(ctx, tx, tag, limit, lessThan, greaterThan)
			if err != nil {
				return err
			}
			flag := true
			var maxid string
			var minid string
			for k := range data {
				if flag {
					maxid = k
					minid = k
					flag = false
				}
				if k > maxid {
					maxid = k
				}
				if k < minid {
					minid = k
				}
			}

			c.JSON(http.StatusOK, tools.Ternary(flag,
				&PaginatedResponse{
					Query:   c.Request.URL.String(),
					Payload: data,
				},
				&PaginatedResponse{
					Query:    c.Request.URL.String(),
					Previous: "/tag/" + tag + "?lt=" + minid,
					Next:     "/tag/" + tag + "?gt=" + maxid,
					Payload:  data,
				}))
			return nil
		})

		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}
	}
}

func PostJsonHandler(db *DBPool) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := c.GetRawData()
		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}

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
						return err
					}
				}
			}
			if v, err := p.ParseBytes(body); err == nil {
				previous := v.GetArray("@metaData", "previous")
				for _, previousID := range previous {
					if err := AddPrevious(ctx, tx, previousID.GetInt64(), id); err != nil {
						return err
					}
				}
			}

			c.JSON(http.StatusOK, map[string]any{"id": strconv.Itoa(int(id))})

			return nil
		})

		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}
	}
}

func Run(addr, connStr string) error {
	if addr == "" || connStr == "" {
		log.Println("addr or connStr is empty", addr, connStr)
		return fmt.Errorf("addr or connStr is empty")
	}

	db, err := NewDBPool(nil, connStr)
	if err != nil {
		log.Println("error on new pool", err.Error())
		return err
	}

	r := gin.Default()

	r.Use(middleware.CORSMiddleware())

	r.POST("/", PostJsonHandler(db))
	r.GET("/:id", GetJsonHandler(db))
	r.GET("/tag/:tag", GetTagHandler(db))

	return r.Run(addr)
}
