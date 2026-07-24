#!/usr/bin/env bash
# Сквозная проверка API на поднятом сервисе.
#
#   make build && sh scripts/smoke.sh
#
# BASE_URL можно переопределить: BASE_URL=http://localhost:9000 sh scripts/smoke.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
USER_A="60601fee-2bf1-4721-ae6f-7636e79a0cba"
USER_B="7f3d1c92-5a48-4b1e-9c77-2e0b6d4a8f31"
MISSING_ID="11111111-1111-4111-8111-111111111111"

passed=0
failed=0

check() {
    local name="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        printf '  ok    %-42s %s\n' "$name" "$actual"
        passed=$((passed + 1))
    else
        printf '  FAIL  %-42s expected=%s actual=%s\n' "$name" "$expected" "$actual"
        failed=$((failed + 1))
    fi
}

# payload service_name price user_id start_date [end_date]
payload() {
    if [ -n "${5:-}" ]; then
        printf '{"service_name":"%s","price":%s,"user_id":"%s","start_date":"%s","end_date":"%s"}' \
            "$1" "$2" "$3" "$4" "$5"
    else
        printf '{"service_name":"%s","price":%s,"user_id":"%s","start_date":"%s"}' "$1" "$2" "$3" "$4"
    fi
}

# send METHOD PATH [BODY] — печатает тело ответа
send() {
    local method="$1" path="$2" body="${3:-}"
    if [ -n "$body" ]; then
        curl -s -X "$method" "$BASE_URL$path" -H 'Content-Type: application/json' -d "$body"
    else
        curl -s -X "$method" "$BASE_URL$path"
    fi
}

# code METHOD PATH [BODY] — печатает HTTP-код
code() {
    local method="$1" path="$2" body="${3:-}"
    if [ -n "$body" ]; then
        curl -s -o /dev/null -w '%{http_code}' -X "$method" "$BASE_URL$path" \
            -H 'Content-Type: application/json' -d "$body"
    else
        curl -s -o /dev/null -w '%{http_code}' -X "$method" "$BASE_URL$path"
    fi
}

field() {
    python3 -c 'import json,sys; print(json.load(sys.stdin).get(sys.argv[1], ""))' "$1"
}

echo "==> health"
check "GET /health" "200" "$(code GET /health)"

echo "==> создание подписок"
body=$(payload "Yandex Plus" 400 "$USER_A" "07-2025")
sub_a=$(send POST /subscriptions "$body")
id_a=$(printf '%s' "$sub_a" | field id)
check "POST /subscriptions (бессрочная)" "07-2025" "$(printf '%s' "$sub_a" | field start_date)"

body=$(payload "Netflix" 900 "$USER_A" "07-2025" "10-2025")
sub_b=$(send POST /subscriptions "$body")
id_b=$(printf '%s' "$sub_b" | field id)
check "POST /subscriptions (с end_date)" "10-2025" "$(printf '%s' "$sub_b" | field end_date)"

body=$(payload "Spotify" 300 "$USER_B" "08-2025")
sub_c=$(send POST /subscriptions "$body")
id_c=$(printf '%s' "$sub_c" | field id)
check "POST /subscriptions (другой юзер)" "300" "$(printf '%s' "$sub_c" | field price)"

echo "==> валидация"
body=$(payload "X" -1 "$USER_A" "07-2025")
check "price < 0 -> 400" "400" "$(code POST /subscriptions "$body")"

body=$(payload "X" 100 "nope" "07-2025")
check "user_id не UUID -> 400" "400" "$(code POST /subscriptions "$body")"

body=$(payload "X" 100 "$USER_A" "2025-07")
check "start_date как YYYY-MM -> 400" "400" "$(code POST /subscriptions "$body")"

body=$(payload "X" 100 "$USER_A" "07-2025" "06-2025")
check "end_date раньше start_date -> 400" "400" "$(code POST /subscriptions "$body")"

check "битый JSON -> 400" "400" "$(code POST /subscriptions '{"service_name":')"

echo "==> чтение"
check "GET /subscriptions/{id}" "Yandex Plus" "$(send GET "/subscriptions/$id_a" | field service_name)"
check "GET несуществующий -> 404" "404" "$(code GET "/subscriptions/$MISSING_ID")"
check "GET невалидный id -> 400" "400" "$(code GET /subscriptions/not-a-uuid)"

echo "==> список и фильтры"
check "GET /subscriptions" "3" "$(send GET /subscriptions | field count)"
check "фильтр по user_id" "2" "$(send GET "/subscriptions?user_id=$USER_A" | field count)"
check "фильтр по service_name (регистр)" "1" "$(send GET '/subscriptions?service_name=yandex%20plus' | field count)"
check "пагинация limit=1" "1" "$(send GET '/subscriptions?limit=1' | field count)"

echo "==> стоимость за 07-2025..12-2025"
# Yandex Plus: 6 мес × 400 = 2400; Netflix: 4 мес × 900 = 3600; Spotify: 5 мес × 300 = 1500
check "всего" "7500" "$(send GET '/subscriptions/cost?from=07-2025&to=12-2025' | field total)"
check "по user_id" "6000" "$(send GET "/subscriptions/cost?from=07-2025&to=12-2025&user_id=$USER_A" | field total)"
check "по service_name" "2400" "$(send GET '/subscriptions/cost?from=07-2025&to=12-2025&service_name=Yandex%20Plus' | field total)"
check "по user_id + service_name" "3600" "$(send GET "/subscriptions/cost?from=07-2025&to=12-2025&user_id=$USER_A&service_name=Netflix" | field total)"
check "один месяц 07-2025" "1300" "$(send GET '/subscriptions/cost?from=07-2025&to=07-2025' | field total)"
check "период вне подписок" "0" "$(send GET '/subscriptions/cost?from=01-2020&to=12-2020' | field total)"
check "to раньше from -> 400" "400" "$(code GET '/subscriptions/cost?from=12-2025&to=07-2025')"
check "без from/to -> 400" "400" "$(code GET /subscriptions/cost)"

echo "==> обновление"
body=$(payload "Yandex Plus" 500 "$USER_A" "07-2025" "09-2025")
updated=$(send PUT "/subscriptions/$id_a" "$body")
check "PUT меняет цену" "500" "$(printf '%s' "$updated" | field price)"
check "PUT меняет end_date" "09-2025" "$(printf '%s' "$updated" | field end_date)"
check "стоимость после PUT" "6600" "$(send GET '/subscriptions/cost?from=07-2025&to=12-2025' | field total)"

body=$(payload "X" 1 "$USER_A" "07-2025")
check "PUT несуществующий -> 404" "404" "$(code PUT "/subscriptions/$MISSING_ID" "$body")"

echo "==> удаление"
check "DELETE -> 204" "204" "$(code DELETE "/subscriptions/$id_a")"
check "DELETE повторно -> 404" "404" "$(code DELETE "/subscriptions/$id_a")"
code DELETE "/subscriptions/$id_b" > /dev/null
code DELETE "/subscriptions/$id_c" > /dev/null
check "список пуст" "0" "$(send GET /subscriptions | field count)"

echo "==> swagger"
check "GET /swagger/index.html" "200" "$(code GET /swagger/index.html)"
check "GET /swagger/doc.json" "200" "$(code GET /swagger/doc.json)"

echo
echo "passed: $passed, failed: $failed"
[ "$failed" -eq 0 ]
