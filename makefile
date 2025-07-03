# Makefile 中的目標如果和檔案名稱相同，就會被當作檔案依賴進行比對。
# .PHONY 宣告表示這是一個純命令執行目標，而非檔案產出，因此每次都會執行。
# 避免因檔案快取或更新時間問題而跳過執行。
.PHONY: dev init proto help install-bin clean

# ===== go proto 設置 =====

ifeq ($(shell uname), Darwin)
    PROTOC_OS = osx
else ifeq ($(shell uname), Linux)
    PROTOC_OS = linux
else
    $(error Unsupported OS)
endif

GOMODULE := yt.com/backend/common.git/business/grpc
PROTOC_VERSION := 26.0
PROTOC_URI := https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-x86_64.zip

BIN_DIR := $(CURDIR)/bin
PROTO_DIR := $(CURDIR)/../proto
OUT_DIR := $(CURDIR)/proto
IMPORT_PREFIX := match/proto

PROTOC := $(BIN_DIR)/protoc
GEN_GO := $(BIN_DIR)/protoc-gen-go
GEN_GRPC := $(BIN_DIR)/protoc-gen-go-grpc

# 啟動應用程式
dev:  
	go run cmd/main.go

# 安裝依賴
init:  
	go mod tidy

# 建置
build:  
	go build -ldflags "-X main.gitCommit=$(git rev-parse --short HEAD)" -o match

# 容器化建置
docker: 
	docker build -t match:latest .

# 安裝工具
install-bin: $(PROTOC) $(GEN_GO) $(GEN_GRPC)

$(PROTOC):
	mkdir -p $(BIN_DIR)	&& \
	curl -sL $(PROTOC_URI) -o /tmp/protoc.zip && \
	unzip -oXq /tmp/protoc.zip -d /tmp/protoc && \
	mv -f /tmp/protoc/bin/protoc $(PROTOC) && \
	mv -f /tmp/protoc/include $(BIN_DIR)/ && \
	rm -rf /tmp/protoc.zip /tmp/protoc

$(GEN_GO):
	GOBIN=$(BIN_DIR) go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0

$(GEN_GRPC):
	GOBIN=$(BIN_DIR) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

# 生成 pb.go
proto:
	@for file in $(PROTO_DIR)/*.proto; do \
		proto_name=$$(basename $$file); \
		echo "Generating $$proto_name"; \
		$(PROTOC) \
			-I $(PROTO_DIR) \
			-I $(BIN_DIR)/include \
			--plugin=protoc-gen-go=$(GEN_GO) \
			--plugin=protoc-gen-go-grpc=$(GEN_GRPC) \
			--go_out=$(OUT_DIR) \
			--go-grpc_out=require_unimplemented_servers=false:$(OUT_DIR) \
			--go_opt=paths=source_relative \
			--go-grpc_opt=paths=source_relative \
			--go_opt=M$$proto_name=$(IMPORT_PREFIX) \
			--go-grpc_opt=M$$proto_name=$(IMPORT_PREFIX) \
			$$file; \
	done

# 清除產出
clean:
	rm -rf $(OUT_DIR)/*.pb.go
	rm -rf $(BIN_DIR)

# 顯示幫助訊息
help:  
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
