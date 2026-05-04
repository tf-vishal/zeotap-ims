#!/bin/bash
# Load test: 5 components × 100 signals each = 500 total signals.
# All signals have the same signal_type to test debouncing aggregation.

API_URL="http://localhost:8080/api/v1/signals"
SIGNAL_TYPE="error"
SEVERITY="critical"
COMPONENTS=("svc-auth" "svc-payments" "svc-orders" "svc-notifications" "svc-inventory")
SIGNALS_PER_COMPONENT=100

echo "═══════════════════════════════════════════════════════════"
echo "  IMS Load Test — ${#COMPONENTS[@]} components × $SIGNALS_PER_COMPONENT signals"
echo "═══════════════════════════════════════════════════════════"
echo ""

total_sent=0
total_accepted=0
total_rejected=0

for component in "${COMPONENTS[@]}"; do
    accepted=0
    rejected=0
    echo "→ Sending $SIGNALS_PER_COMPONENT signals for component: $component"

    for i in $(seq 1 $SIGNALS_PER_COMPONENT); do
        timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.%3NZ")
        response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API_URL" \
            -H "Content-Type: application/json" \
            -d "{
                \"component_id\": \"$component\",
                \"signal_type\": \"$SIGNAL_TYPE\",
                \"severity\": \"$SEVERITY\",
                \"timestamp\": \"$timestamp\",
                \"metadata\": {\"index\": $i, \"source\": \"load-test\"}
            }")

        if [ "$response" = "202" ]; then
            ((accepted++))
        else
            ((rejected++))
        fi
    done

    echo "  ✓ $component: accepted=$accepted rejected=$rejected"
    total_sent=$((total_sent + SIGNALS_PER_COMPONENT))
    total_accepted=$((total_accepted + accepted))
    total_rejected=$((total_rejected + rejected))
done

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  RESULTS"
echo "═══════════════════════════════════════════════════════════"
echo "  Total sent:     $total_sent"
echo "  Total accepted: $total_accepted"
echo "  Total rejected: $total_rejected"
echo ""

# Wait for debounce windows to close (10s window + some margin)
echo "⏳ Waiting 15 seconds for debounce windows to flush..."
sleep 15

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  VERIFICATION"
echo "═══════════════════════════════════════════════════════════"

# Check MongoDB signal count
echo ""
echo "📦 MongoDB signal_audits count:"
docker exec ims-mongo mongosh --quiet --eval "db.signal_audits.countDocuments({})" ims 2>/dev/null || echo "  (check manually)"

# Check PostgreSQL work items
echo ""
echo "🗄️  PostgreSQL work_items:"
PGPASSWORD=ims_secret psql -h localhost -p 5433 -U ims -d ims -c "SELECT id, component_id, status, signal_count, first_seen_at, last_seen_at FROM work_items ORDER BY component_id;" 2>/dev/null || echo "  (check manually with: psql -h localhost -p 5433 -U ims -d ims)"

# Check processor health
echo ""
echo "💚 Processor health:"
curl -s http://localhost:8081/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8081/health
echo ""
