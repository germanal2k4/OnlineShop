curl -X POST http://localhost:9000/orders \
  -u admin:secret \
  -H "Content-Type: application/json" \
  -d '{
    "id": "order123",
    "recipient_id": "user42",
    "storage_deadline": "2025-01-01T12:00:00Z",
    "weight": 3.5,
    "cost": 100.0,
    "packaging": ["box", "film"]
}'

curl -X GET "http://localhost:9000/orders?cursor=order001&limit=2"

curl -X GET http://localhost:9000/orders/order001


curl -X PUT http://localhost:9000/orders/order001 \
  -u admin:secret \
  -H "Content-Type: application/json" \
  -d '{
    "id": "order001",
    "recipient_id": "user999",
    "storage_deadline": "2025-05-01T10:00:00Z",
    "weight": 6.7,
    "cost": 300,
    "packaging": ["bag"]
}'
