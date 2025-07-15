import random
import string
from locust import HttpUser, task, between

class AssetUser(HttpUser):
    wait_time = between(0.5, 1)

    def on_start(self):
        self.token = ""

    def random_uid(self):
        return ''.join(random.choices(string.ascii_letters + string.digits, k=9))

    @task(3)
    def add_asset(self):
        headers = {
            # 可自行開啟需要的 Header
            "Content-Type": "application/json"
        }
        payload = {
            "uid": self.random_uid(),
            "currency": "USD",
            "amount": round(random.uniform(1, 1000), 2)
        }
        self.client.post("/asset/add", json=payload, headers=headers)

    @task(1)
    def get_balances(self):
        headers = {
            # 可自行開啟需要的 Header
            "Content-Type": "application/json"
        }
        self.client.get("/asset/balances", headers=headers)
