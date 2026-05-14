#!/bin/bash

# Test script to verify adding content and listing pins
# This tests the full flow: add -> list -> verify

set -e

CLUSTER_URL="${CLUSTER_URL:-http://localhost:9094}"

echo "========================================="
echo "Testing Add and List Functionality"
echo "========================================="
echo ""

# Create a test file
TEST_FILE="/tmp/test_metadata_$(date +%s).txt"
echo "Test content for metadata testing - $(date)" > "$TEST_FILE"
echo "Created test file: $TEST_FILE"
echo ""

# Create metadata JSON with uploadedBy
METADATA_JSON='{"uploadedBy":"Divya","testKey":"testValue","timestamp":"'$(date -Iseconds)'"}'

echo "Test 1: Adding content with metadata..."
echo "Metadata: $METADATA_JSON"
echo ""

RESPONSE=$(curl -s -X POST \
  -F "file=@$TEST_FILE" \
  -F "metadata=$METADATA_JSON" \
  "$CLUSTER_URL/add?name=test_metadata_file.txt" \
  -u "admin:j2JNCkhivunT2EjhbhjBQ5QQ60GLxwbDl9ai/7sP3bY=")

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

# Wait a moment for pinning to complete
echo "Waiting 3 seconds for pinning to complete..."
sleep 3
echo ""

# Test 2: List all pins
echo "Test 2: Listing all pins..."
echo "Making request to: $CLUSTER_URL/pins"
PINS_LIST=$(curl -v -s "$CLUSTER_URL/pins" -u "admin:j2JNCkhivunT2EjhbhjBQ5QQ60GLxwbDl9ai/7sP3bY=" 2>&1)
HTTP_CODE=$(echo "$PINS_LIST" | grep -oP '< HTTP/\d+\.\d+ \K\d+' | head -1)
echo "HTTP Response Code: $HTTP_CODE"
echo ""

# Extract just the response body (after the headers)
PINS_BODY=$(echo "$PINS_LIST" | sed -n '/^{/,$p')
echo "Pins list response (first 1000 chars):"
echo "$PINS_BODY" | head -c 1000
echo ""
echo ""

# Count how many pins are in the response
PIN_COUNT=$(echo "$PINS_BODY" | jq 'length' 2>/dev/null || echo "0")
echo "Number of pins in response: $PIN_COUNT"
echo ""

# List all CIDs in the response
echo "All CIDs in /pins response:"
echo "$PINS_BODY" | jq -r '.[].cid' 2>/dev/null | head -20 || echo "Could not parse CIDs"
echo ""

# Check if our CID is in the list
if echo "$PINS_BODY" | grep -q "$CID"; then
  echo "✓ SUCCESS: Pin found in /pins listing!"
  echo ""
  
  # Extract the pin info for our CID
  PIN_INFO=$(echo "$PINS_LIST" | jq ".[] | select(.cid == \"$CID\")" 2>/dev/null || echo "")
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
  echo "  Looking for CID: $CID"
  echo ""
  
  # Try with filter=pinned
  echo "Trying with filter=pinned..."
  PINS_FILTERED=$(curl -s "$CLUSTER_URL/pins?filter=pinned" -u "admin:j2JNCkhivunT2EjhbhjBQ5QQ60GLxwbDl9ai/7sP3bY=")
  if echo "$PINS_FILTERED" | grep -q "$CID"; then
    echo "  ✓ Found in /pins?filter=pinned"
  else
    echo "  ✗ NOT found in /pins?filter=pinned"
  fi
  echo ""
  
  echo "Checking pin status directly..."
  PIN_STATUS=$(curl -s "$CLUSTER_URL/pins/$CID" -u "admin:j2JNCkhivunT2EjhbhjBQ5QQ60GLxwbDl9ai/7sP3bY=")
  echo "Pin status for $CID:"
  echo "$PIN_STATUS" | jq '.' 2>/dev/null || echo "$PIN_STATUS"
  echo ""
  
  # Extract status from direct query
  DIRECT_STATUS=$(echo "$PIN_STATUS" | jq -r '.peer_map | to_entries[0].value.status' 2>/dev/null || echo "unknown")
  echo "Direct status query shows: $DIRECT_STATUS"
  echo ""
fi

echo ""
echo "========================================="
echo "Test completed!"
echo "========================================="

# Cleanup
rm -f "$TEST_FILE"

