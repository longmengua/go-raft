# 使用官方的 locustio/locust 基礎映像
FROM locustio/locust

# 設置工作目錄
WORKDIR /mnt/locust

# 拷貝當前目錄的 locustfile.py 到容器內
COPY locustfile.py .

# 設置容器啟動時的命令
ENTRYPOINT ["locust"]
