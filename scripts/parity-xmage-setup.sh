#!/usr/bin/env bash
# scripts/parity-xmage-setup.sh — automated xmage adapter setup helper.
#
# Companion to docs/cross-engine-parity-runbook-r60.md. Provides three
# modes for the Phase 4 operator:
#
#   --preflight       Probe machine for Go / Python / JDK / Maven / disk.
#                     Exit 0 if all present, 1 if any missing. Prints a
#                     structured report.
#
#   --build-and-cache Clone xmage at the pinned tag, build the
#                     fat-jar via mvn, drop it at ~/.cache/hexdek/.
#                     Sets HEXDEK_XMAGE_JAR in the user's shell rc.
#                     Idempotent — skips clone/build if already cached.
#
#   --check-jar       Verify the configured HEXDEK_XMAGE_JAR exists and
#                     is invokable. Smoke-runs `java -jar $JAR --version`.
#
# Exit codes:
#   0  success
#   1  missing prerequisite
#   2  network / build failure
#   77 explicitly opted out (--skip-on-no-jvm + no JVM)

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." &> /dev/null && pwd)"

XMAGE_TAG="${XMAGE_TAG:-xmage_1.4.55}"
XMAGE_SRC_DIR="${XMAGE_SRC_DIR:-$HOME/src/mage}"
XMAGE_CACHE_DIR="${XMAGE_CACHE_DIR:-$HOME/.cache/hexdek}"
XMAGE_JAR_CACHE="${XMAGE_CACHE_DIR}/xmage-${XMAGE_TAG#xmage_}.jar"

usage() {
    sed -n '2,20p' "$0"
    exit 0
}

# ---------------------------------------------------------------------
# Preflight — probe machine for the prerequisites listed in the runbook.
# ---------------------------------------------------------------------
preflight() {
    local fail=0

    probe() {
        local label="$1"; shift
        local out
        if out="$("$@" 2>&1)"; then
            printf "[preflight] %-8s %s\n" "$label:" "$(echo "$out" | head -1)"
        else
            printf "[preflight] %-8s MISSING (\`%s\` errored)\n" "$label:" "$1"
            fail=1
        fi
    }

    probe "Go"      go version
    probe "python3" python3 --version
    # `java` on macOS may be a stub; check by parsing the output.
    local jv
    jv="$(java -version 2>&1 || true)"
    if echo "$jv" | grep -qi "unable to locate"; then
        printf "[preflight] %-8s MISSING (macOS stub — install OpenJDK 17 per runbook §1.1)\n" "java:"
        fail=1
    elif echo "$jv" | head -1 | grep -qiE "version|openjdk"; then
        printf "[preflight] %-8s %s\n" "java:" "$(echo "$jv" | head -1)"
    else
        printf "[preflight] %-8s UNCERTAIN (\"%s\")\n" "java:" "$(echo "$jv" | head -1)"
        fail=1
    fi
    probe "mvn"     mvn --version
    probe "git"     git --version

    local avail_gb
    avail_gb="$(df -g "${HOME}" 2>/dev/null | awk 'NR==2 {print $4}')"
    if [[ -n "$avail_gb" ]]; then
        printf "[preflight] %-8s %s GB free at \$HOME\n" "disk:" "$avail_gb"
        if (( avail_gb < 5 )); then
            printf "[preflight] %-8s WARNING: under 5 GB, xmage build may fail\n" "disk:"
        fi
    fi

    if (( fail == 0 )); then
        echo "[preflight] OK — all prerequisites present."
        return 0
    else
        echo "[preflight] FAIL — see runbook §1 for install steps."
        return 1
    fi
}

# ---------------------------------------------------------------------
# Build-and-cache — full setup. Requires preflight to pass first.
# ---------------------------------------------------------------------
build_and_cache() {
    if ! preflight >/dev/null 2>&1; then
        echo "[build] preflight failed; aborting. Re-run with --preflight to see what's missing." >&2
        return 1
    fi

    mkdir -p "$XMAGE_CACHE_DIR"

    if [[ -f "$XMAGE_JAR_CACHE" ]]; then
        echo "[build] cache hit at $XMAGE_JAR_CACHE — skipping clone/build."
    else
        if [[ ! -d "$XMAGE_SRC_DIR/.git" ]]; then
            echo "[build] cloning xmage to $XMAGE_SRC_DIR ..."
            mkdir -p "$(dirname "$XMAGE_SRC_DIR")"
            git clone --depth 1 --branch "$XMAGE_TAG" \
                https://github.com/magefree/mage.git "$XMAGE_SRC_DIR" \
                || { echo "[build] clone failed (network?)" >&2; return 2; }
        else
            echo "[build] xmage source already at $XMAGE_SRC_DIR — fetching $XMAGE_TAG ..."
            (cd "$XMAGE_SRC_DIR" && git fetch --depth 1 origin "$XMAGE_TAG" && git checkout "$XMAGE_TAG") \
                || { echo "[build] fetch/checkout failed" >&2; return 2; }
        fi

        echo "[build] running mvn package (this takes ~10 min on first run) ..."
        (
            cd "$XMAGE_SRC_DIR" && \
            mvn -DskipTests=true -pl Mage.Server -am package assembly:single
        ) || { echo "[build] mvn package failed" >&2; return 2; }

        local fat_jar
        fat_jar="$(find "$XMAGE_SRC_DIR/Mage.Server/target" \
            -name 'mage-server-*-jar-with-dependencies.jar' 2>/dev/null | head -1)"
        if [[ -z "$fat_jar" ]]; then
            echo "[build] BUILD SUCCESS but no fat-jar found in Mage.Server/target/" >&2
            return 2
        fi
        cp "$fat_jar" "$XMAGE_JAR_CACHE"
        echo "[build] cached at $XMAGE_JAR_CACHE"
    fi

    # Append to user's shell rc if HEXDEK_XMAGE_JAR isn't already set.
    if [[ -z "${HEXDEK_XMAGE_JAR:-}" ]]; then
        local rc
        rc="${HOME}/.$(basename "${SHELL:-bash}")rc"
        if [[ -f "$rc" ]] && ! grep -q "HEXDEK_XMAGE_JAR" "$rc"; then
            echo "" >> "$rc"
            echo "# Added by scripts/parity-xmage-setup.sh" >> "$rc"
            echo "export HEXDEK_XMAGE_JAR=\"$XMAGE_JAR_CACHE\"" >> "$rc"
            echo "[build] HEXDEK_XMAGE_JAR exported in $rc — restart shell to pick up."
        else
            echo "[build] set this in your shell rc:"
            echo "    export HEXDEK_XMAGE_JAR=\"$XMAGE_JAR_CACHE\""
        fi
    fi

    echo "[build] done. Next: scripts/parity-test.sh --tag combat-fundamentals"
}

# ---------------------------------------------------------------------
# Check-jar — verify the configured jar is invokable.
# ---------------------------------------------------------------------
check_jar() {
    local jar="${HEXDEK_XMAGE_JAR:-$XMAGE_JAR_CACHE}"
    if [[ ! -f "$jar" ]]; then
        echo "[check-jar] FAIL — jar not found at $jar" >&2
        return 1
    fi
    if ! java -jar "$jar" --version >/dev/null 2>&1; then
        # Many adapter shims don't implement --version; that's fine.
        # We're really just probing for a usable JVM + jar manifest.
        if ! java -jar "$jar" --help >/dev/null 2>&1 && \
           ! jar tf "$jar" >/dev/null 2>&1; then
            echo "[check-jar] FAIL — jar at $jar is not invokable by current java" >&2
            return 1
        fi
    fi
    local sz
    sz="$(du -m "$jar" 2>/dev/null | awk '{print $1}')"
    echo "[check-jar] OK — $jar ($sz MB)"
}

# ---------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------
case "${1:-}" in
    --preflight)        preflight ;;
    --build-and-cache)  build_and_cache ;;
    --check-jar)        check_jar ;;
    -h|--help|"")       usage ;;
    *)
        echo "Unknown flag: $1" >&2
        usage
        exit 1
        ;;
esac
