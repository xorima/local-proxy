#!/usr/bin/env bash
set -euo pipefail

SQUID_URL="http://localhost:13128"

echo "=== Test 1: Direct access (no auth) - should fail ==="
curl -s -o /dev/null -w "%{http_code}" --proxy "$SQUID_URL" "http://example.com" || echo " (expected: no auth fails)"

echo ""
echo "=== Test 2: Auth with wrong password - should fail ==="
curl -s -o /dev/null -w "%{http_code}" --proxy "$SQUID_URL" --proxy-user "user:wrongpass" "http://example.com" || echo " (expected: bad auth fails)"

echo ""
echo "=== Test 3: Auth with correct user:pass - should succeed ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --proxy "$SQUID_URL" --proxy-user "user:pass" "http://example.com")
echo "HTTP $HTTP_CODE (expected: 200)"

echo ""
echo "=== Test 4: HTTPS through proxy with auth - should succeed ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --proxy "$SQUID_URL" --proxy-user "user:pass" "https://example.com")
echo "HTTP $HTTP_CODE (expected: 200)"

echo ""
echo "Done."
