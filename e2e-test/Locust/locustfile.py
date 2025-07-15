from locust import HttpUser, task, between
import random

class AssetUser(HttpUser):
    wait_time = between(0.5, 1)  # 每個請求間隔 0.5-1 秒

    def on_start(self):
        """初始化動作"""
        self.token = ""  # 如果需要 Token，這裡填寫
        self.uids = ["user1", "user2", "user3"]  # 可自訂測試帳號

    @task(3)  # 權重 3，表示比 balances 呼叫頻率高
    def add_asset(self):
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Accept": "application/json",
            "Content-Type": "application/json",
            "User-Agent": "LocustLoadTest/1.0"
        }
        payload = {
            "uid": random.choice(self.uids),
            "currency": "USD",
            "amount": round(random.uniform(1, 1000), 2)
        }
        self.client.post("/asset/add", json=payload, headers=headers)

    @task(1)  # 權重 1，表示大約每 4 次有 1 次是查詢
    def get_balances(self):
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Accept": "application/json",
            "User-Agent": "LocustLoadTest/1.0"
        }
        self.client.get("/asset/balances", headers=headers)
