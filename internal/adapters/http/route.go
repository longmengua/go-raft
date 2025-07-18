package http

import (
	"fmt"
	"go-raft/internal/adapters/http/asset"
	"go-raft/internal/adapters/http/snapshot"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type HttpServer struct {
	Addr            []string
	assethandler    *asset.Handler
	snapshothandler *snapshot.Handler
}

func New(addr []string, assethandler *asset.Handler, snapshothandler *snapshot.Handler) *HttpServer {
	return &HttpServer{
		Addr:            addr,
		assethandler:    assethandler,
		snapshothandler: snapshothandler,
	}
}

func (hs *HttpServer) Start() error {
	r := gin.Default()

	// CORS middleware (放在最前面)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // 本地測試用，正式環境要限定
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		AllowCredentials: true,
	}))

	// 設置靜態文件服務
	r.Static("/static", "./static")

	// 設置日誌格式
	gin.DefaultWriter = NewLogger()

	// 設置全局中間件
	r.Use(NewRequestTimeout(5 * time.Second))
	r.Use(NewTraceID()) // 添加Trace ID中間件

	// Asset相關路由
	r.POST("/asset/add", hs.assethandler.AddAsset)
	r.GET("/asset/balance", hs.assethandler.GetBalance)
	r.GET("/asset/balances", hs.assethandler.GetBalances)

	// 切換 snapshot 版本號，滾動更新用
	r.GET("/snapshot/version", hs.snapshothandler.GetSnapshotVersion)
	r.POST("/snapshot/version", hs.snapshothandler.SetSnapshotVersion)

	// 啟動HTTP服務器
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	if len(hs.Addr) == 0 {
		return fmt.Errorf("no address provided")
	}
	if len(hs.Addr) > 1 {
		return fmt.Errorf("not support multiple address")
	}

	r.Run(hs.Addr[0])
	log.Printf("HTTP server started on %s", hs.Addr)
	return nil
}
