#!/bin/bash
API_KEY="DatS4UH1Leyg9qG2Kc"
API_SECRET="UlUKA1qDeVRa3XYAuzDbGVyWqZZICWvvmY6y"
RECV_WINDOW="5000"
BASE_URL="https://api-demo.bybit.com"
TIMESTAMP=$(date +%s)000
PATH="/v5/position/closed-pnl?category=linear&limit=20"
PAYLOAD="${TIMESTAMP}${API_KEY}${RECV_WINDOW}category=linear&limit=20"
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$API_SECRET" | awk '{print $2}')
curl -s -H "X-BAPI-API-KEY: $API_KEY" \
     -H "X-BAPI-TIMESTAMP: $TIMESTAMP" \
     -H "X-BAPI-SIGN: $SIGNATURE" \
     -H "X-BAPI-RECV-WINDOW: $RECV_WINDOW" \
     "${BASE_URL}${PATH}"