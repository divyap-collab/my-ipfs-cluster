#!/usr/bin/env bash
# Test metadata filter on /pins endpoint.
# Usage:
#   ./test_metadata_filter.sh [BASE_URL] [AUTH]
#   BASE_URL defaults to http://localhost:9094
#   AUTH defaults to env CLUSTER_CREDS (user:pass) or no auth
#
# Example with auth:
#   CLUSTER_CREDS='admin:yourpassword' ./test_metadata_filter.sh
#   ./test_metadata_filter.sh http://localhost:9094 'admin:yourpassword'

set -e
BASE_URL="${1:-http://localhost:9094}"
AUTH="${2:-$CLUSTER_CREDS}"

CURL_OPTS=(-s -S)
if [[ -n "$AUTH" ]]; then
  CURL_OPTS+=(-u "$AUTH")
fi

echo "=== Testing metadata filter at $BASE_URL ==="
echo ""

# 1. Get count without filter
echo "1. GET /pins (no filter)..."
RESP_ALL=$(curl "${CURL_OPTS[@]}" "$BASE_URL/pins")
COUNT_ALL=$(echo "$RESP_ALL" | grep -c '"cid":' || true)
echo "   Found $COUNT_ALL pins"
echo ""

# 2. Get with metadata filter patient.id:P-hospital-1
echo "2. GET /pins?metadata=patient.id:P-hospital-1 ..."
RESP_FILTERED=$(curl "${CURL_OPTS[@]}" "$BASE_URL/pins?metadata=patient.id:P-hospital-1")
COUNT_FILTERED=$(echo "$RESP_FILTERED" | grep -c '"cid":' || true)
echo "   Found $COUNT_FILTERED pins"
echo ""

# 3. Check if filtered response only contains patient.id=P-hospital-1
echo "3. Checking filtered results have patient.id=P-hospital-1..."
if [[ "$COUNT_FILTERED" -eq 0 ]]; then
  echo "   No pins returned (filter may be too strict or no matching pins)"
else
  # Each pin's metadata should contain "patient": {"id": "P-hospital-1"
  BAD=$(echo "$RESP_FILTERED" | grep -c '"patient": {}' || true)
  if [[ "$BAD" -gt 0 ]]; then
    echo "   FAIL: $BAD pin(s) in filtered response have empty patient (should be excluded)"
    exit 1
  fi
  # Check we have P-hospital-1 in the response (simple check)
  if echo "$RESP_FILTERED" | grep -q '"id": "P-hospital-1"'; then
    echo "   OK: Filtered response contains pins with patient.id=P-hospital-1"
  fi
fi
echo ""

# 4. Verdict
if [[ "$COUNT_FILTERED" -lt "$COUNT_ALL" ]] || [[ "$COUNT_ALL" -eq 0 ]]; then
  echo "=== Result: Filter is applied (filtered=$COUNT_FILTERED, total=$COUNT_ALL) ==="
else
  echo "=== Result: Filter may NOT be applied (filtered=$COUNT_FILTERED same as total=$COUNT_ALL) ==="
  echo "   If you have pins with patient.id=P-hospital-1 and others without, filtered count should be smaller."
  exit 1
fi
