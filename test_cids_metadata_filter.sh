#!/usr/bin/env bash
# Test GET /pins?cids=...&metadata=... on ipfs-cluster.
# Expectation: only pins matching the metadata filter are returned.

set -e

BASE="${1:-http://localhost:9094}"
CREDS="${CLUSTER_CREDS:-admin:j2JNCkhivunT2EjhbhjBQ5QQ60GLxwbDl9ai/7sP3bY=}"

# Exact URL from user: cids (2 CIDs) + metadata=observation.bloodOxygen:96
CIDS="bafkreic367viypvh37ahlwylz66c73quvhqjk3e42sipmxk6sw4q6t36v4,bafkreiem7va2ysu3szexprkga5wl6xceumgxzrldddon7pdbfgwnl45gwe"
METADATA="observation.bloodOxygen%3A96"
URL="${BASE}/pins?cids=${CIDS}&metadata=${METADATA}"

echo "=== Testing CID + metadata filter at ${BASE} ==="
echo "URL: ${URL}"
echo ""

echo "1. GET /pins with cids + metadata=observation.bloodOxygen:96..."
RESPONSE=$(curl -s -u "$CREDS" "$URL")
COUNT=$(echo "$RESPONSE" | grep -c '"cid":' || true)

echo "   Found $COUNT pin(s)"
echo ""

echo "2. Checking that all returned pins have observation.bloodOxygen = 96..."
ALL_96=true
BLOOD_OXYGEN_VALS=""
while IFS= read -r line; do
  [ -z "$line" ] && continue
  val=$(echo "$line" | jq -r '.metadata.observation.bloodOxygen // empty' 2>/dev/null)
  if [ -n "$val" ]; then
    BLOOD_OXYGEN_VALS="$BLOOD_OXYGEN_VALS $val"
    if [ "$val" != "96" ]; then
      ALL_96=false
      echo "   FAIL: found bloodOxygen = $val (expected 96)"
    fi
  fi
done <<< "$RESPONSE"

if [ -z "$BLOOD_OXYGEN_VALS" ]; then
  echo "   WARN: could not extract observation.bloodOxygen from response"
  echo "   First 500 chars of response:"
  echo "$RESPONSE" | head -c 500
  echo ""
elif [ "$ALL_96" = true ]; then
  echo "   OK: all pins have observation.bloodOxygen = 96"
else
  echo "   Listed bloodOxygen values:$BLOOD_OXYGEN_VALS"
fi

echo ""
echo "3. Without metadata filter (both CIDs should be returned)..."
URL_NO_FILTER="${BASE}/pins?cids=${CIDS}"
RESPONSE_NO_FILTER=$(curl -s -u "$CREDS" "$URL_NO_FILTER")
COUNT_NO_FILTER=$(echo "$RESPONSE_NO_FILTER" | grep -c '"cid":' || true)
echo "   Found $COUNT_NO_FILTER pin(s) (expected 2)"

echo ""
if [ "$COUNT" -eq 1 ] && [ "$ALL_96" = true ]; then
  echo "=== Result: Filter is working (1 pin with bloodOxygen=96, filter applied) ==="
elif [ "$COUNT" -eq 2 ] && [ "$COUNT_NO_FILTER" -eq 2 ]; then
  echo "=== Result: Filter may NOT be applied (2 pins returned same as no filter) ==="
  exit 1
else
  echo "=== Result: Filter returned $COUNT pin(s); expected 1 with bloodOxygen=96 ==="
  [ "$ALL_96" != true ] && exit 1
fi
