#!/bin/bash
# ============================================================
# test_flow.sh — End-to-End Test Script for Mini P2P Lending Engine
# Boots docker-compose, resets PostgreSQL, reapplies init.sql, then
# verifies the full loan state machine and investment variants.
#
# Requirements: docker compose, curl, grep, sed, awk
# No jq dependency!
# ============================================================

set -eu

BASE_URL="http://localhost:8080/api/v1"
COMPOSE_PROJECT="loan-engine-e2e"
COMPOSE_CMD="docker compose -p ${COMPOSE_PROJECT}"
VISIT_FILE="/tmp/visit_proof.txt"
AGREEMENT_FILE="/tmp/agreement.pdf"

# ============================================================
# Colors & Formatting
# ============================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

print_header() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}${CYAN}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

print_success() {
    echo -e "  ${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "  ${RED}✗ $1${NC}"
}

print_info() {
    echo -e "  ${YELLOW}→ $1${NC}"
}

print_response() {
    echo -e "  ${CYAN}Response:${NC} $1"
}

cleanup() {
    rm -f "$VISIT_FILE" "$AGREEMENT_FILE"
}

trap cleanup EXIT

# ============================================================
# Helper: Extract a JSON field value (works for both int and string)
# Usage: extract_field "response_body" "field_name"
# Handles nested JSON by extracting the FIRST occurrence.
# ============================================================
extract_field() {
    local body="$1"
    local field="$2"
    echo "$body" | sed -n "s/.*\"${field}\":[[:space:]]*\([0-9][0-9]*\).*/\1/p" | head -1
}

# ============================================================
# Helper: Check HTTP status code
# Usage: check_status "$http_code" "expected_code" "step_description"
# ============================================================
check_status() {
    local actual="$1"
    local expected="$2"
    local description="$3"

    if [ "$actual" != "$expected" ]; then
        print_error "FAILED: $description"
        print_error "Expected HTTP $expected, got HTTP $actual"
        exit 1
    fi
}

api_request() {
    local expected_status="$1"
    local description="$2"
    shift 2

    local response
    response=$(curl -sS -w "\n%{http_code}" "$@")

    HTTP_CODE=$(printf '%s\n' "$response" | tail -1)
    BODY=$(printf '%s\n' "$response" | sed '$d')

    check_status "$HTTP_CODE" "$expected_status" "$description"
    print_response "$BODY"
}

assert_loan_in_state() {
    local loan_id="$1"
    local state="$2"

    print_info "GET $BASE_URL/loans?state=$state"
    api_request "200" "List loans in state $state" -X GET "$BASE_URL/loans?state=$state"

    if echo "$BODY" | grep -q "\"id\":$loan_id"; then
        print_success "Loan $loan_id confirmed in state ${state^^}"
    else
        print_error "Loan $loan_id not found in state ${state^^}"
        exit 1
    fi
}

create_product() {
    print_header "SETUP: Create Product"
    print_info "POST $BASE_URL/products"

    api_request "201" "Create Product" -X POST "$BASE_URL/products" \
        -H "Content-Type: application/json" \
        -d '{
            "product_name": "Personal Loan Gold",
            "tenor_length": 12,
            "payment_frequency": "MONTHLY",
            "interest_rate": 10.50,
            "roi_rate": 8.00,
            "penalty_rate": 2.00,
            "min_principal_amount": 1000,
            "max_principal_amount": 100000
        }'

    PRODUCT_ID=$(extract_field "$BODY" "id")
    if [ -z "$PRODUCT_ID" ]; then
        print_error "Failed to extract product_id from response"
        exit 1
    fi

    print_success "Product created successfully (product_id=$PRODUCT_ID)"
}

create_loan() {
    local principal_amount="$1"

    print_info "POST $BASE_URL/loans"
    print_info "borrower_id=1, product_id=$PRODUCT_ID, principal_amount=$principal_amount"

    api_request "201" "Create Loan" -X POST "$BASE_URL/loans" \
        -H "Content-Type: application/json" \
        -d "{
            \"borrower_id\": 1,
            \"product_id\": $PRODUCT_ID,
            \"principal_amount\": $principal_amount
        }"

    CREATED_LOAN_ID=$(extract_field "$BODY" "id")
    if [ -z "$CREATED_LOAN_ID" ]; then
        print_error "Failed to extract loan_id from response"
        exit 1
    fi

    print_success "Loan created successfully (loan_id=$CREATED_LOAN_ID, status=PROPOSED)"
    assert_loan_in_state "$CREATED_LOAN_ID" "proposed"
}

approve_loan() {
    local loan_id="$1"

    print_info "POST $BASE_URL/loans/$loan_id/approve"
    print_info "staff_id=4, uploading visit proof picture"

    api_request "200" "Approve Loan" -X POST "$BASE_URL/loans/$loan_id/approve" \
        -F "staff_id=4" \
        -F "picture=@$VISIT_FILE"

    if echo "$BODY" | grep -q '"date_approval":null'; then
        print_error "Approval response should include date_approval"
        exit 1
    fi

    print_success "Loan approved by staff_id=4 (loan_id=$loan_id)"
    assert_loan_in_state "$loan_id" "approved"
}

invest_loan() {
    local loan_id="$1"
    local investor_id="$2"
    local amount="$3"
    local label="$4"

    print_info "$label"
    api_request "200" "Invest Loan" -X POST "$BASE_URL/loans/$loan_id/invest" \
        -H "Content-Type: application/json" \
        -d "{
            \"investor_id\": $investor_id,
            \"amount\": $amount
        }"
}

disburse_loan() {
    local loan_id="$1"

    print_info "POST $BASE_URL/loans/$loan_id/disburse"
    print_info "staff_id=5, uploading signed agreement document"

    api_request "200" "Disburse Loan" -X POST "$BASE_URL/loans/$loan_id/disburse" \
        -F "staff_id=5" \
        -F "agreement=@$AGREEMENT_FILE"

    if echo "$BODY" | grep -q '"date_disbursed":null'; then
        print_error "Disburse response should include date_disbursed"
        exit 1
    fi

    print_success "Loan disbursed by staff_id=5 (loan_id=$loan_id)"
    assert_loan_in_state "$loan_id" "disbursed"
}

wait_for_app() {
    print_header "PRE-FLIGHT: Boot Docker Stack"
    print_info "Resetting containers and volumes so init.sql runs from a clean database"
    $COMPOSE_CMD down -v --remove-orphans >/dev/null 2>&1 || true

    print_info "Starting postgres, rabbitmq, and app containers"
    $COMPOSE_CMD up --build -d

    print_info "Waiting for API to become ready"
    local attempts=0
    while true; do
        if curl -sS -o /dev/null "$BASE_URL/loans?state=approved"; then
            break
        fi

        attempts=$((attempts + 1))
        if [ "$attempts" -ge 60 ]; then
            print_error "App did not become ready in time"
            $COMPOSE_CMD ps
            exit 1
        fi
        sleep 2
    done

    print_success "Docker stack is ready and init.sql has been applied"
}

# ============================================================
print_header "PRE-FLIGHT: Setup"

printf '%s\n' "dummy visit proof photo content" > "$VISIT_FILE"
printf '%s\n' "dummy signed agreement document" > "$AGREEMENT_FILE"
print_success "Created dummy upload files in /tmp/"

wait_for_app
create_product

print_header "TEST CASE 1: Many Investors -> Loan becomes INVESTED"
create_loan "10000"
MANY_INVESTORS_LOAN_ID="$CREATED_LOAN_ID"
approve_loan "$MANY_INVESTORS_LOAN_ID"

print_header "CASE 1A: Partial funding keeps loan in APPROVED"
invest_loan "$MANY_INVESTORS_LOAN_ID" "2" "3000" "Investor 2 funds 3000"
print_success "Loan partially funded and should remain APPROVED"
assert_loan_in_state "$MANY_INVESTORS_LOAN_ID" "approved"

print_header "CASE 1B: More investors complete funding -> INVESTED"
invest_loan "$MANY_INVESTORS_LOAN_ID" "3" "4000" "Investor 3 funds 4000"
assert_loan_in_state "$MANY_INVESTORS_LOAN_ID" "approved"
invest_loan "$MANY_INVESTORS_LOAN_ID" "2" "3000" "Investor 2 tops up final 3000"

echo ""
echo -e "  ${YELLOW}Wait for outbox worker to process RabbitMQ event...${NC}"
sleep 2

assert_loan_in_state "$MANY_INVESTORS_LOAN_ID" "invested"
disburse_loan "$MANY_INVESTORS_LOAN_ID"

print_header "TEST CASE 2: One Investor -> Loan becomes INVESTED"
create_loan "7000"
ONE_INVESTOR_LOAN_ID="$CREATED_LOAN_ID"
approve_loan "$ONE_INVESTOR_LOAN_ID"
invest_loan "$ONE_INVESTOR_LOAN_ID" "2" "7000" "Investor 2 funds full amount 7000"
sleep 2
assert_loan_in_state "$ONE_INVESTOR_LOAN_ID" "invested"
disburse_loan "$ONE_INVESTOR_LOAN_ID"

print_header "TEST CASE 3: Partial Investment -> Loan stays APPROVED"
create_loan "9000"
PARTIAL_LOAN_ID="$CREATED_LOAN_ID"
approve_loan "$PARTIAL_LOAN_ID"
invest_loan "$PARTIAL_LOAN_ID" "3" "4000" "Investor 3 funds 4000 of 9000"
print_success "Loan still has remainder amount and should stay APPROVED"
assert_loan_in_state "$PARTIAL_LOAN_ID" "approved"

print_header "NEGATIVE TESTS: State Machine Guards"

print_info "Attempting to approve disbursed loan $MANY_INVESTORS_LOAN_ID again"
response=$(curl -sS -w "\n%{http_code}" -X POST "$BASE_URL/loans/$MANY_INVESTORS_LOAN_ID/approve" \
    -F "staff_id=4" \
    -F "picture=@$VISIT_FILE")
HTTP_CODE=$(printf '%s\n' "$response" | tail -1)
BODY=$(printf '%s\n' "$response" | sed '$d')
if [ "$HTTP_CODE" = "422" ]; then
    print_success "Correctly rejected re-approve on disbursed loan"
    print_response "$BODY"
else
    print_error "Expected HTTP 422, got HTTP $HTTP_CODE"
    exit 1
fi

print_info "Attempting to invest in disbursed loan $ONE_INVESTOR_LOAN_ID"
response=$(curl -sS -w "\n%{http_code}" -X POST "$BASE_URL/loans/$ONE_INVESTOR_LOAN_ID/invest" \
    -H "Content-Type: application/json" \
    -d '{"investor_id": 2, "amount": 1000}')
HTTP_CODE=$(printf '%s\n' "$response" | tail -1)
BODY=$(printf '%s\n' "$response" | sed '$d')
if [ "$HTTP_CODE" = "422" ]; then
    print_success "Correctly rejected investing in disbursed loan"
    print_response "$BODY"
else
    print_error "Expected HTTP 422, got HTTP $HTTP_CODE"
    exit 1
fi

print_info "Attempting approve with non-staff user"
create_loan "5000"
NON_STAFF_LOAN_ID="$CREATED_LOAN_ID"
response=$(curl -sS -w "\n%{http_code}" -X POST "$BASE_URL/loans/$NON_STAFF_LOAN_ID/approve" \
    -F "staff_id=2" \
    -F "picture=@$VISIT_FILE")
HTTP_CODE=$(printf '%s\n' "$response" | tail -1)
BODY=$(printf '%s\n' "$response" | sed '$d')
if [ "$HTTP_CODE" = "422" ]; then
    print_success "Correctly rejected non-STAFF approval"
    print_response "$BODY"
else
    print_error "Expected HTTP 422, got HTTP $HTTP_CODE"
    exit 1
fi

print_header "ALL TESTS PASSED"
echo ""
echo -e "  ${GREEN}${BOLD}Verified scenarios:${NC}"
echo -e "  ${GREEN}  1. Loan funded by many investors -> INVESTED -> DISBURSED${NC}"
echo -e "  ${GREEN}  2. Loan funded by one investor -> INVESTED -> DISBURSED${NC}"
echo -e "  ${GREEN}  3. Loan partially funded -> remains APPROVED${NC}"
echo ""
echo -e "  ${CYAN}Summary:${NC}"
echo -e "    • Product created:                    id=$PRODUCT_ID"
echo -e "    • Many-investor loan:                 id=$MANY_INVESTORS_LOAN_ID"
echo -e "    • One-investor loan:                  id=$ONE_INVESTOR_LOAN_ID"
echo -e "    • Partial-investment loan:            id=$PARTIAL_LOAN_ID"
echo -e "    • Negative guard tests:               passed"
echo -e "    • Stack remains running in Docker     project=$COMPOSE_PROJECT"
echo ""
