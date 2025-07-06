package http

import (
	"fmt"
	"go-raft/internal/adapters/http/asset"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

type HttpServer struct {
	Addr         []string
	assethandler *asset.Handler
}

func New(addr []string, assethandler *asset.Handler) *HttpServer {
	return &HttpServer{
		Addr:         addr,
		assethandler: assethandler,
	}
}

func (hs *HttpServer) Start() error {
	r := gin.Default()

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
