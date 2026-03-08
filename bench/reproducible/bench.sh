#!/bin/bash
# Cross-runtime benchmark: Kruda vs Fiber
# Usage:
#   bash bench_all.sh                    # run all routes
#   bash bench_all.sh plaintext          # run only plaintext
#   bash bench_all.sh db queries         # run db + queries

set -e
WRK="wrk -t4 -c256 -d5s"
CORES="taskset -c 0-7"
LUA="/tmp/post_json.lua"
DATABASE_URL="${DATABASE_URL:-postgres://benchmarkdbuser:benchmarkdbpass@localhost:5432/hello_world?pool_max_conns=64&pool_min_conns=8}"
export DATABASE_URL

cat > $LUA << 'EOF'
wrk.method = "POST"
wrk.body   = '{"name":"bench","value":42.5}'
wrk.headers["Content-Type"] = "application/json"
EOF

pkill -9 -f kruda-bench 2>/dev/null || true
pkill -9 -f fiber-bench 2>/dev/null || true
sleep 1

ROUTES="${@:-plaintext param postjson json db queries fortunes updates}"

run_route() {
    local port=$1 route=$2
    case $route in
        plaintext) printf "  %-12s" "plaintext:"; $WRK http://localhost:$port/ 2>&1 | grep 'Requests/sec' ;;
        param)     printf "  %-12s" "param GET:"; $WRK http://localhost:$port/users/42 2>&1 | grep 'Requests/sec' ;;
        postjson)  printf "  %-12s" "POST JSON:"; $WRK -s $LUA http://localhost:$port/json 2>&1 | grep 'Requests/sec' ;;
        json)      printf "  %-12s" "JSON GET:";  $WRK http://localhost:$port/json 2>&1 | grep 'Requests/sec' ;;
        db)        printf "  %-12s" "db:";        $WRK http://localhost:$port/db 2>&1 | grep 'Requests/sec' ;;
        queries)   printf "  %-12s" "queries:";   $WRK "http://localhost:$port/queries?q=20" 2>&1 | grep 'Requests/sec' ;;
        fortunes)  printf "  %-12s" "fortunes:";  $WRK http://localhost:$port/fortunes 2>&1 | grep 'Requests/sec' ;;
        updates)   printf "  %-12s" "updates:";   $WRK "http://localhost:$port/updates?q=20" 2>&1 | grep 'Requests/sec' ;;
    esac
}

run_all() {
    local port=$1
    for route in $ROUTES; do
        run_route $port $route
    done
}

echo "============================================"
echo "  Kruda vs Fiber"
echo "============================================"

echo ""
echo "--- Kruda ---"
$CORES env GOMAXPROCS=8 KRUDA_WORKERS=8 PORT=3000 DATABASE_URL="$DATABASE_URL" ~/kruda/bench/cross-runtime/kruda/kruda-bench &
sleep 2
run_all 3000
pkill -9 -f kruda-bench 2>/dev/null; sleep 1

echo ""
echo "--- Fiber ---"
$CORES env GOMAXPROCS=8 PORT=3002 DATABASE_URL="$DATABASE_URL" ~/kruda/bench/cross-runtime/fiber/fiber-bench &
sleep 2
run_all 3002
pkill -9 -f fiber-bench 2>/dev/null; sleep 1

echo ""
echo "============================================"
echo "  Done — $(date '+%Y-%m-%d %H:%M:%S %Z')"
echo "============================================"
