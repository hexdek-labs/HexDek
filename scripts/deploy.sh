#!/bin/bash
# HexDek deploy script — builds and deploys to DARKSTAR + MISTY
# Usage: ./scripts/deploy.sh [backend|frontend|both|frontend-dev]
#
# Targets:
#   backend       — cross-compile + ship hexdek-server to DARKSTAR (prod API)
#   frontend      — build + rsync React app to MISTY's ~/sites/hexdek/   (prod: hexdek.dev)
#   frontend-dev  — build + rsync React app to MISTY's ~/sites/hexdek-dev/ (staging: dev.hexdek.dev)
#   both          — backend + frontend (prod)
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

    echo "=== Swapping binary + restarting via setsid -f ==="
    # ssh -n detaches stdin, and setsid -f forks the remote start-hexdek.sh into
    # its own session so ssh isn't waiting on inherited FDs from the server's
    # children. The </dev/null >/dev/null 2>&1 redirection severs every FD the
    # ssh remote shell would otherwise wait on, which is the combination that
    # makes this call return immediately.
    #
    # Important: do NOT pkill -f hexdek-server from here. The ssh remote shell's
    # own command line contains the literal string "hexdek-server", so pkill -f
    # matches and kills the parent shell, breaking the ssh session (exit 255).
    # start-hexdek.sh handles stopping the previous server on its own.
    ssh -n "$DARKSTAR" 'set -e; mv /tmp/hexdek-server-new "$HOME/hexdek/hexdek-server" && chmod +x "$HOME/hexdek/hexdek-server" && setsid -f "$HOME/hexdek/start-hexdek.sh" </dev/null >/dev/null 2>&1'

    echo "=== Waiting for :8090 to come up ==="
    local tries=0
    local max_tries=20
    until ssh -n -o ConnectTimeout=5 "$DARKSTAR" "ss -tln 2>/dev/null | grep -q ':8090 '" 2>/dev/null; do
        tries=$((tries + 1))
        if [ "$tries" -ge "$max_tries" ]; then
            echo "ERROR: hexdek-server did not bind :8090 within $((max_tries * 2))s"
            ssh -n "$DARKSTAR" 'tail -30 /tmp/hexdek-server.log 2>/dev/null || true'
            rm -f hexdek-server-linux
            return 1
        fi
        sleep 2
    done

    echo "=== Verifying ==="
    ssh -n "$DARKSTAR" 'ss -tlnp 2>/dev/null | grep :8090; echo "---"; tail -3 /tmp/hexdek-server.log'
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

deploy_frontend_dev() {
    # Staging deploy — built WITH the staging review gate (VITE_STAGING=1:
    # only the whitelisted reviewers in src/lib/stagingGate.js get past
    # the gate; everyone else is redirected to prod). Lands in a separate
    # directory on MISTY served at dev.hexdek.dev. Both prod and dev
    # point at the same backend API on DARKSTAR :8090, so this is a
    # visual-layer staging environment only. Prod builds (deploy_frontend)
    # never set the flag and are byte-for-byte unaffected by the gate.
    local branch
    branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    echo "=== Building React frontend — STAGING GATE ON (branch: $branch) ==="
    cd hexdek && VITE_STAGING=1 npm run build && cd ..

    echo "=== Deploying to MISTY (~/sites/hexdek-dev/) ==="
    rsync -avz --delete hexdek/dist/ "$MISTY:~/sites/hexdek-dev/"

    echo "=== Dev frontend deploy complete — live at dev.hexdek.dev ==="
}

case "$TARGET" in
    backend)      deploy_backend ;;
    frontend)     deploy_frontend ;;
    frontend-dev) deploy_frontend_dev ;;
    both)         deploy_backend; deploy_frontend ;;
    *)            echo "Usage: $0 [backend|frontend|both|frontend-dev]"; exit 1 ;;
esac
