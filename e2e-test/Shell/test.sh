#!/bin/bash

BASE_URL="http://localhost:9090"
REQUESTS_PER_API=${1:-1000}    # 每個 API 執行請求數
CONCURRENT=${2:-10}            # 同時併發數
OUTPUT_FILE="statistic.csv"

API_PATHS=(
  "/asset/add"
  # "/asset/balances"
)

echo "request_num,api_path,response_time_sec" > $OUTPUT_FILE

# 產生所有待發送的 api_path 清單，每個 API 重複 REQUESTS_PER_API 次
declare -a request_list=()

for api_path in "${API_PATHS[@]}"; do
  for ((i=0; i<REQUESTS_PER_API; i++)); do
    request_list+=("$api_path")
  done
done

# shuffle 陣列函式 (Fisher-Yates 洗牌演算法)
shuffle() {
  local i tmp size max rand
  size=${#request_list[@]}
  max=$(( 32768 / size * size ))
  for ((i=size-1; i>0; i--)); do
    while (( (rand=RANDOM) >= max )); do :; done
    rand=$(( rand % (i+1) ))
    tmp=${request_list[i]}
    request_list[i]=${request_list[rand]}
    request_list[rand]=$tmp
  done
}

shuffle

echo "開始發送 $(( REQUESTS_PER_API * ${#API_PATHS[@]} )) 筆請求，併發數 $CONCURRENT ..."

request_counter=0

# 記錄開始時間
start_time=$(date +%s.%N)

random_string() {
  local chars=abcdefghijklmnopqrstuvwxyz0123456789
  local str=""
  for i in {1..6}; do
    str+=${chars:RANDOM%${#chars}:1}
  done
  echo "$str"
}

random_currency() {
  local arr=(USDT USDC BTC ETH)
  echo "${arr[RANDOM % ${#arr[@]}]}"
}

random_amount() {
  awk -v min=0 -v max=1000 'BEGIN{srand(); printf "%.2f", min+rand()*(max-min)}'
}

function run_request() {
  local req_num=$1
  local api_path=$2

  if [ "$api_path" == "/asset/add" ]; then
    local uid="$(random_string)"
    local currency=$(random_currency)
    local amount=$(random_amount)
    payload=$(jq -n --arg uid "$uid" --arg currency "$currency" --argjson amount "$amount" \
      '{uid: $uid, currency: $currency, amount: $amount}')
    method="POST"
  else
    payload=""
    method="GET"
  fi

  if [ "$method" == "POST" ]; then
    response_time=$(curl -s -o /dev/null -w "%{time_total}" -X POST "$BASE_URL$api_path" \
      -H "Content-Type: application/json" \
      -d "$payload")
  else
    response_time=$(curl -s -o /dev/null -w "%{time_total}" "$BASE_URL$api_path")
  fi

  echo "$req_num,$api_path,$response_time" >> $OUTPUT_FILE
  echo "Request $req_num ($api_path): ${response_time}s"
}

total_requests=${#request_list[@]}
current_req=0

for api_path in "${request_list[@]}"; do
  ((current_req++))

  # 控制併發數
  while [ $(jobs -r | wc -l) -ge $CONCURRENT ]; do
    sleep 0.05
  done

  run_request $current_req "$api_path" &

done

wait

end_time=$(date +%s.%N)
total_duration=$(echo "$end_time - $start_time" | bc)

echo "全部請求完成，總共花費時間: ${total_duration} 秒"

# 統計
awk -F, '
NR>1 {
  api=$2;
  sum[api]+=$3;
  count[api]++;
  if((api in min)==0 || $3<min[api]) min[api]=$3;
  if((api in max)==0 || $3>max[api]) max[api]=$3;
}
END {
  for(a in count){
    printf("API %s\n", a);
    printf("  共計請求數: %d\n", count[a]);
    printf("  平均回應時間: %.4f 秒\n", sum[a]/count[a]);
    printf("  最短回應時間: %.4f 秒\n", min[a]);
    printf("  最長回應時間: %.4f 秒\n", max[a]);
    print "";
  }
}
' $OUTPUT_FILE
