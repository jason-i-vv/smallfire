#!/bin/bash
# 测试 MiniMax AI API 调用（用于调试 AIKeyLevelAnalyzer 的 401 错误）
# 用法: ./scripts/test_ai_api.sh [API_KEY]

set -e

API_KEY="${1:-$AI_API_KEY}"

if [ -z "$API_KEY" ]; then
    echo "Error: AI_API_KEY not set. Pass as argument or set AI_API_KEY env var."
    echo "Usage: $0 [API_KEY]"
    exit 1
fi

BASE_URL="https://api.minimaxi.com/v1"
MODEL="MiniMax-M2.7-highspeed"
TEMPERATURE=0.3
MAX_TOKENS=2000

# 简化的 system prompt（与 key_level_analyzer.go 一致）
SYSTEM_PROMPT='你是专业的技术分析师。根据提供的K线数据，识别关键支撑位和阻力位。
只输出一个JSON对象，不要输出其他内容。不要用markdown代码块包裹。

输出格式：
{"levels":[{"price":85200.00,"type":"resistance","strength":8,"reason":"3次测试+整数关口","source_type":"round_number"}]}

规则：
1. 只识别真正的关键价位，不要报告每个小波动
2. strength 评分：1-3(弱), 4-6(中), 7-10(强)
3. source_type: round_number, multi_test, swing_point, volume_cluster, historical
4. 每侧最多5个价位
5. 价格精确到小数点后2位'

# 简化的 user prompt
USER_PROMPT='标的: BTCUSDT
周期: 1h
当前价: 85432.50

近期K线(最近20根):
  04-13 10:00 O:84800.00 H:85500.00 L:84600.00 C:85200.00 Vol:1234560 (+0.47%)
  04-13 11:00 O:85200.00 H:85700.00 L:85000.00 C:85432.50 Vol:1345670 (+0.27%)

当前均线: EMA30=85100.00 EMA60=84800.00 EMA90=84500.00
20周期均量: 1234567
近期区间: 最高=86000.00 最低=84000.00 振幅=2.38%

请识别当前关键支撑位和阻力位。'

echo "=== Testing MiniMax AI API ==="
echo "Base URL: $BASE_URL"
echo "Model: $MODEL"
echo ""

# 构建请求
REQUEST_BODY=$(cat <<EOF
{
  "model": "$MODEL",
  "messages": [
    {"role": "system", "content": $(echo "$SYSTEM_PROMPT" | jq -Rs .)},
    {"role": "user", "content": $(echo "$USER_PROMPT" | jq -Rs .)}
  ],
  "max_tokens": $MAX_TOKENS,
  "temperature": $TEMPERATURE,
  "response_format": {"type": "json_object"}
}
EOF
)

echo "Request body:"
echo "$REQUEST_BODY" | jq .
echo ""

# 发送请求
echo "Sending request..."
RESPONSE=$(curl -s -w "\n\nHTTP_STATUS:%{http_code}" \
    -X POST "${BASE_URL}/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${API_KEY}" \
    -d "$REQUEST_BODY")

HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | sed 's/HTTP_STATUS://')
BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')

echo ""
echo "HTTP Status: $HTTP_STATUS"
echo "Response:"
echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
