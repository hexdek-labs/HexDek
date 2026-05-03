#!/bin/bash
# HexDek deploy script — builds and deploys to DARKSTAR + MISTY
# Usage: ./scripts/deploy.sh [backend|frontend|both]
set -euo pipefail

DARKSTAR="josh@192.168.1.207"
MISTY="josh@192.168.1.200"
TARGET="${1:-both}"

deploy_backend() {
    echo "=== Building hexdek-server for Linux amd64 ==="
    GOOS=linux GOARCH=amd64 go build -o hexdek-server-linux ./cmd/hexdek-server/
    echo "Binary: $(ls -lh hexdek-server-linux | awk '{print $5}')"

    echo "=== Uploading to DARKSTAR ==="
    scp hexdek-server-linux "$DARKSTAR:/tmp/hexdek-server-new"

    echo "=== Swapping binary + restarting on port 8090 ==="
    ssh "$DARKSTAR" 'pkill -f hexdek-server || true; sleep 1; mv /tmp/hexdek-server-new $HOME/hexdek/hexdek-server && chmod +x $HOME/hexdek/hexdek-server && $HOME/hexdek/start-hexdek.sh'

    echo "=== Verifying ==="
    sleep 3
    ssh "$DARKSTAR" 'ss -tlnp | grep 8090 && tail -3 /tmp/hexdek-server.log'
    rm -f hexdek-server-linux
    echo "=== Backend deploy complete ==="
}

deploy_frontend() {
    echo "=== Building React frontend ==="
    cd hexdek && npm run build && cd ..

    echo "=== Deploying to MISTY (~/sites/hexdek/) ==="
    # CRITICAL: source is hexdek/dist/ (Vite React build), NOT web/ (old plain HTML MVP)
    # Target is ~/sites/hexdek/, NOT ~/hexdek/
    rsync -avz --delete hexdek/dist/ "$MISTY:~/sites/hexdek/"

    echo "=== Frontend deploy complete ==="
}

case "$TARGET" in
    backend)  deploy_backend ;;
    frontend) deploy_frontend ;;
    both)     deploy_backend; deploy_frontend ;;
    *)        echo "Usage: $0 [backend|frontend|both]"; exit 1 ;;
esac
