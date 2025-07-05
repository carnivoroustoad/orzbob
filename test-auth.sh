#!/bin/bash
# Test script for GitHub authentication flow

set -e

echo "🧪 Testing GitHub Authentication Flow"
echo "===================================="

# Set test environment
export GITHUB_CLIENT_ID="test-client-id"
export ORZBOB_API_URL="http://localhost:8080"

# Check if API server is running
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "❌ API server not running. Starting it..."
    ./bin/cloud-cp &
    API_PID=$!
    sleep 2
    
    # Cleanup on exit
    trap "kill $API_PID 2>/dev/null || true" EXIT
fi

echo "✅ API server is running"

# Test 1: Health check (no auth required)
echo -e "\n1️⃣ Testing health endpoint (no auth)..."
curl -s http://localhost:8080/health | jq .

# Test 2: Try to access protected endpoint without auth
echo -e "\n2️⃣ Testing protected endpoint without auth..."
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/v1/instances)
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" = "401" ]; then
    echo "✅ Correctly rejected: $BODY"
else
    echo "❌ Expected 401, got $HTTP_CODE"
fi

# Test 3: Test auth exchange endpoint
echo -e "\n3️⃣ Testing auth exchange endpoint..."
EXCHANGE_RESPONSE=$(curl -s -X POST http://localhost:8080/v1/auth/exchange \
    -H "Content-Type: application/json" \
    -d '{
        "github_token": "ghp_test123",
        "github_id": 12345,
        "github_login": "testuser",
        "email": "test@example.com"
    }')

# For testing, the exchange will fail since we're not using a real GitHub token
echo "Exchange response: $EXCHANGE_RESPONSE"

# Test 4: Test with mock Bearer token
echo -e "\n4️⃣ Testing with Bearer token..."
# This will fail without a valid JWT, but shows the auth flow
curl -s -H "Authorization: Bearer mock-jwt-token" \
    http://localhost:8080/v1/user | jq . || echo "Expected failure with mock token"

echo -e "\n✅ Basic authentication flow tests completed!"
echo "To test full flow, you need:"
echo "1. A real GitHub OAuth App Client ID"
echo "2. Run: ./bin/orz login"
echo "3. Then: ./bin/orz cloud new"