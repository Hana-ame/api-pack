package shijima

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/Hana-ame/api-pack/shijima/handler"
	"github.com/Hana-ame/api-pack/shijima/middleware"
	"github.com/Hana-ame/api-pack/shijima/repo"
	handlerpkg "github.com/Hana-ame/api-pack/utils/my_gin_handler"
	middlewarepkg "github.com/Hana-ame/api-pack/utils/my_gin_middleware"
	_ "github.com/Hana-ame/api-pack/utils/utils"
	tools "github.com/Hana-ame/api-pack/utils/utils"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func init() {
	dbPath := tools.Or(os.Getenv("SHIJIMA_DB"), "shijima.db")
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		panic(fmt.Sprintf("failed to open database: %v", err))
	}
	db.SetMaxOpenConns(1)
}

func Run(addr string) error {
	if addr == "" {
		return nil
	}

	r := repo.New(db)
	if err := r.InitDB(); err != nil {
		return fmt.Errorf("initDB: %w", err)
	}

	h := handler.New(r)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(middlewarepkg.CORSMiddleware())
	router.Use(middlewarepkg.ProxyMiddleware())

	// v2 compat routes
	v2 := router.Group("/api/v2")
	v2.GET("/", h.V2Get)
	v2.POST("/", middleware.CheckID, h.V2Post)
	v2.DELETE("/", middleware.CheckID, h.V2Delete)
	v2.GET("/cookie", handler.CookieHandler)
	v2.GET("/reaction/:tid", middleware.CheckID, h.V2GetReactions)
	v2.POST("/reaction/:tid", middleware.CheckID, h.V2SetReaction)
	v2.GET("/new_reactions", h.GetNewReactionsHandler)

	// v3 routes
	h.RegisterV3Routes(router, middleware.CheckID)

	// preview
	router.GET("/api/v2/preview/*path", h.Preview)

	// static
	wwwRoot := tools.Or(os.Getenv("WWW_ROOT"), "/var/www/moonchan")
	router.NoRoute(handlerpkg.NoRoute(wwwRoot, "index.html"))

	return router.Run(addr)
}
