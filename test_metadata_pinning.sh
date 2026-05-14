#!/bin/bash

# Comprehensive test for metadata extraction and pinning
# Tests the full flow: add with metadata -> verify pin appears in listing

set -e

CLUSTER_URL="${CLUSTER_URL:-http://localhost:9094}"

echo "========================================="
echo "Testing Metadata Extraction and Pinning"
echo "========================================="
echo ""

# Create a test file
TEST_FILE="/tmp/test_metadata_$(date +%s).txt"
echo "Test content for metadata and pinning test - $(date)" > "$TEST_FILE"
echo "Created test file: $TEST_FILE"
echo ""

# Create complex metadata JSON with nested objects and arrays
METADATA_JSON='{
  "uploadedBy": "Divya",
  "testKey": "testValue",
  "number": 123,
  "boolean": true,
  "nested": {
    "level1": "value1",
    "level2": {
      "level3": "value3"
    }
  },
  "array": [1, 2, 3],
  "mixedArray": ["string", 42, true, {"nested": "object"}]
}'

echo "Test 1: Adding content with complex metadata..."
echo "Metadata: $METADATA_JSON"
echo ""

RESPONSE=$(curl -s -X POST \
  -F "file=@$TEST_FILE" \
  -F "metadata=$METADATA_JSON" \
  "$CLUSTER_URL/add?name=test_metadata_file.txt")

echo "Add Response:"
echo "$RESPONSE" | jq '.' 2>/dev/null || echo "$RESPONSE"
echo ""

# Extract CID from response
CID=$(echo "$RESPONSE" | grep -o '"cid":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
if [ -z "$CID" ]; then
  CID=$(echo "$RESPONSE" | grep -o '"hash":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
fi

if [ -z "$CID" ]; then
  echo "ERROR: Could not extract CID from response"
  echo "Full response: $RESPONSE"
  exit 1
fi

echo "✓ Successfully added content"
echo "  CID: $CID"
echo ""

# Wait for pinning to complete
echo "Waiting 5 seconds for pinning to complete..."
sleep 5
echo ""

# Test 2: List all pins
echo "Test 2: Listing all pins..."
PINS_LIST=$(curl -s "$CLUSTER_URL/pins")

# Check if our CID is in the list
if echo "$PINS_LIST" | grep -q "$CID"; then
  echo "✓ SUCCESS: Pin found in /pins listing!"
  echo ""
  
  # Extract the pin info for our CID
  PIN_INFO=$(echo "$PINS_LIST" | jq ".[] | select(.cid == \"$CID\" or .cid.cid == \"$CID\")" 2>/dev/null || echo "")
  if [ ! -z "$PIN_INFO" ]; then
    echo "Pin details:"
    echo "$PIN_INFO" | jq '.' 2>/dev/null || echo "$PIN_INFO"
    echo ""
    
    # Check for metadata
    if echo "$PIN_INFO" | grep -q "Divya"; then
      echo "✓ SUCCESS: Metadata 'uploadedBy: Divya' found in pin!"
    else
      echo "⚠ WARNING: Metadata not found in pin details"
    fi
  fi
else
  echo "✗ ERROR: Pin NOT found in /pins listing"
  echo ""
  echo "Checking pin status directly..."
  PIN_STATUS=$(curl -s "$CLUSTER_URL/pins/$CID")
  STATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$CLUSTER_URL/pins/$CID")
  
  if [ "$STATUS_CODE" = "200" ]; then
    echo "Pin status for $CID:"
    echo "$PIN_STATUS" | jq '.' 2>/dev/null || echo "$PIN_STATUS"
    echo ""
    echo "⚠ Pin exists but not in listing - possible filter or status issue"
  else
    echo "✗ Pin status check returned HTTP $STATUS_CODE"
    echo "Response: $PIN_STATUS"
  fi
fi

echo ""
echo "Test 3: Checking specific pin endpoint..."
PIN_GET=$(curl -s "$CLUSTER_URL/pins/$CID")
if echo "$PIN_GET" | grep -q "$CID"; then
  echo "✓ Pin can be retrieved directly"
  if echo "$PIN_GET" | grep -q "Divya"; then
    echo "✓ Metadata found in direct pin retrieval"
  fi
else
  echo "✗ Could not retrieve pin directly"
  echo "Response: $PIN_GET"
fi

echo ""
echo "========================================="
echo "Test completed!"
echo "========================================="

# Cleanup
rm -f "$TEST_FILE"

