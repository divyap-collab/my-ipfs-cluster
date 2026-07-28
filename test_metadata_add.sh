#!/bin/bash

# Test script to verify metadata extraction and pinning functionality
# This script tests adding content with metadata in the request body

set -e

echo "Testing metadata extraction and pinning..."

# Create a test file
TEST_FILE="/tmp/test_content.txt"
echo "This is test content for metadata testing" > "$TEST_FILE"

# Create metadata JSON
METADATA_JSON='{"uploadedBy":"Divya","testKey":"testValue","number":123,"array":[1,2,3]}'

# Test 1: Add content with metadata in form body
echo ""
echo "Test 1: Adding content with metadata in form body..."
RESPONSE=$(curl -s -X POST \
  -F "file=@$TEST_FILE" \
  -F "metadata=$METADATA_JSON" \
  "http://localhost:9094/add?name=test_file.txt")

echo "Response: $RESPONSE"

# Extract CID from response (assuming JSON response with cid field)
CID=$(echo "$RESPONSE" | grep -o '"cid":"[^"]*"' | cut -d'"' -f4 || echo "")

if [ -z "$CID" ]; then
  echo "ERROR: Could not extract CID from response"
  exit 1
fi

echo "Successfully added content with CID: $CID"

# Wait a moment for pinning to complete
sleep 2

# Test 2: Verify the pin appears in the listing
echo ""
echo "Test 2: Checking if pin appears in /pins listing..."
PINS_LIST=$(curl -s "http://localhost:9094/pins")

if echo "$PINS_LIST" | grep -q "$CID"; then
  echo "SUCCESS: Pin found in listing!"
else
  echo "WARNING: Pin not found in listing"
  echo "Listing response: $PINS_LIST"
fi

# Test 3: Check pin status and metadata
echo ""
echo "Test 3: Checking pin status and metadata..."
PIN_STATUS=$(curl -s "http://localhost:9094/pins/$CID")

if echo "$PIN_STATUS" | grep -q "Divya"; then
  echo "SUCCESS: Metadata 'uploadedBy: Divya' found in pin status!"
else
  echo "WARNING: Metadata not found in pin status"
  echo "Status response: $PIN_STATUS"
fi

echo ""
echo "Test completed!"

