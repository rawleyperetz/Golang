#!/bin/bash
 
BASE_URL="http://localhost:8080"
PASS_ONE="password123"
PASS_TWO="password456"
USER_ONE="Alice"
USER_TWO="Bob"


echo "========================================="
echo " Chatroom API Test Script"
echo "========================================="
 
# ── Create Room ───────────────────────────────
echo ""
echo "[1] Creating room..."
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/rooms" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_ONE\", \"passwd\": \"$PASS_ONE\"}")
 
echo "Response: $CREATE_RESPONSE"
ROOM_ID=$CREATE_RESPONSE
 
if [ -z "$ROOM_ID" ]; then
  echo "FAIL: No room ID returned"
  exit 1
fi
echo "PASS: Room created with ID: $ROOM_ID"
 
# ── Create Room - Missing Fields ──────────────
echo ""
echo "[2] Creating room with missing fields (expect 400)..."
BAD_CREATE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rooms" \
  -H "Content-Type: application/json" \
  -d '{"name": ""}')
 
if [ "$BAD_CREATE" -eq 400 ]; then
  echo "PASS: Got 400 as expected"
else
  echo "FAIL: Expected 400, got $BAD_CREATE"
fi

# ── Wait Handler (background) ─────────────────
echo ""
echo "[5] Calling wait endpoint in background..."
curl -s -X GET "$BASE_URL/rooms/$ROOM_ID/wait" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_ONE\", \"passwd\": \"$PASS_ONE\"}" &

WAIT_PID=$!
echo "Wait handler running in background (PID: $WAIT_PID)"

# give the wait handler time to register before join fires
sleep 1

# ── Join Room ─────────────────────────────────
echo ""
echo "[3] Joining room $ROOM_ID..."
JOIN_RESPONSE=$(curl -s -X POST "$BASE_URL/rooms/$ROOM_ID/join" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_TWO\", \"passwd\": \"$PASS_ONE\"}")

echo "Join Response: $JOIN_RESPONSE"

# wait for the background wait handler to finish
echo ""
echo "Waiting for wait handler to complete..."
wait $WAIT_PID
echo "Wait handler finished"
 
# # ── Wait Handler ──────────────────────────────
# echo ""
# echo "[5] Calling wait endpoint (expect both IPs connected)..."
# WAIT_RESPONSE=$(curl -s -X GET "$BASE_URL/rooms/$ROOM_ID/wait" \
#   -H "Content-Type: application/json" \
#   -d "{\"name\": \"testroom\", \"username\":\"$USER_ONE\", \"passwd\": \"$PASS_ONE\"}")
#  
# echo "Response: $WAIT_RESPONSE"
# 
# 
# # ── Join Room ─────────────────────────────────
# echo ""
# echo "[3] Joining room $ROOM_ID..."
# JOIN_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rooms/$ROOM_ID/join" \
#   -H "Content-Type: application/json" \
#   -d "{\"name\": \"testroom\", \"username\":\"$USER_TWO\", \"passwd\": \"$PASS_ONE\"}")
#  
# if [ "$JOIN_RESPONSE" -eq 200 ]; then
#   echo "PASS: Joined room successfully"
# else
#   echo "FAIL: Expected 200, got $JOIN_RESPONSE"
# fi
 
# ── Join Room - Wrong Password ─────────────────
echo ""
echo "[4] Joining room with wrong password (expect 401)..."
BAD_JOIN=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rooms/$ROOM_ID/join" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_TWO\", \"passwd\": \"wrongpassword\"}")
 
if [ "$BAD_JOIN" -eq 401 ]; then
  echo "PASS: Got 401 as expected"
else
  echo "FAIL: Expected 401, got $BAD_JOIN"
fi
 

 
# ── Post Message ──────────────────────────────
echo ""
echo "[6] Posting a message..."
MSG_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rooms/$ROOM_ID/messages" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_TWO\", \"passwd\": \"$PASS_ONE\", \"msg\": \"hello\"}")
 
if [ "$MSG_RESPONSE" -eq 200 ]; then
  echo "PASS: Message posted successfully"
else
  echo "FAIL: Expected 200, got $MSG_RESPONSE"
fi
 
# ── Post Message - Missing Fields ─────────────
echo ""
echo "[7] Posting message with missing msg field (expect 400)..."
BAD_MSG=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rooms/$ROOM_ID/messages" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_TWO\", \"passwd\": \"$PASS_ONE\", \"msg\": \"\"}")
 
if [ "$BAD_MSG" -eq 400 ]; then
  echo "PASS: Got 400 as expected"
else
  echo "FAIL: Expected 400, got $BAD_MSG"
fi
 
# ── Get Messages ──────────────────────────────
echo ""
echo "[8] Getting messages..."
GET_MSG_RESPONSE=$(curl -s -X GET "$BASE_URL/rooms/$ROOM_ID/messages" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_ONE\", \"passwd\": \"$PASS_ONE\"}")
 
echo "Response: $GET_MSG_RESPONSE"
 
# ── Get Messages - Wrong Password ─────────────
echo ""
echo "[9] Getting messages with wrong password (expect 401)..."
BAD_GET_MSG=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/rooms/$ROOM_ID/messages" \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"testroom\", \"username\":\"$USER_ONE\", \"passwd\": \"wrongpassword\"}")
 
if [ "$BAD_GET_MSG" -eq 401 ]; then
  echo "PASS: Got 401 as expected"
else
  echo "FAIL: Expected 401, got $BAD_GET_MSG"
fi
 
# ── Get All Rooms ─────────────────────────────
echo ""
echo "[10] Getting all rooms..."
ROOMS_RESPONSE=$(curl -s -X GET "$BASE_URL/rooms")
echo "Response: $ROOMS_RESPONSE"
 
echo ""
echo "========================================="
echo " Tests Complete"
echo "========================================="
 
