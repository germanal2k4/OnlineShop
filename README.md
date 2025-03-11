## CURL-запросы

Создаёт новый заказ – аналог “acceptOrder”.
```bash
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
```

Возвращает список заказов, сортируя по id, с курсорной пагинацией.
```bash
curl -X GET "http://localhost:9000/orders?cursor=order001&limit=2"
```

Возвращает один заказ по id.
```bash
curl -X GET "http://localhost:9000/orders/order123"
```

Позволяет изменить поля заказа (ID в теле должен совпадать с id в URL).
```bash
curl -X PUT "http://localhost:9000/orders/order123" \
-u admin:secret \
-H "Content-Type: application/json" \
-d '{
"id": "order123",
"recipient_id": "user999",
"storage_deadline": "2025-07-01T12:00:00Z",
"packaging": ["bag"],
"weight": 6.5,
"cost": 250
}'
```

Позволяет изменить поля заказа (ID в теле должен совпадать с id в URL).

```bash
curl -X DELETE "http://localhost:9000/orders/order123" \
  -u admin:secret
```

Переводит заказ в состояние «delivered».
```bash
curl -X PUT "http://localhost:9000/orders-deliver/order123" \
  -u admin:secret
```

Переводит заказ в состояние «client_rtn» (возврат от клиента).

```bash
curl -X PUT "http://localhost:9000/orders-return/order123" \
  -u admin:secret
```

Возвращает заказы, которые в состоянии «client_rtn», с пагинацией через offset/limit.

```bash
curl -X GET "http://localhost:9000/returns?offset=0&limit=10"
```
