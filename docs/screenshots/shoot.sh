#!/usr/bin/env bash
# Regenerates the README screenshots: `just screenshots`.
#
# Runs pholio against the demo Vault in ./vault inside a private tmux server,
# captures each screen as ANSI, and renders it to PNG with Charm's freeze.
#
# The Vault's dates are relative: @D0@ is today, @D-3@ three days ago, @W0@
# today's weekday, and @S-1@ yesterday's Zettel stamp prefix (YYYYMMDD).
# They are filled in at run time so today's Daily Note and the Task List's
# Overdue/Today/Upcoming groups look the same whenever this runs.
#
# Needs tmux, python3, and freeze on PATH (the just recipe supplies freeze).
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
sock="pholio-shots-$$"
t() { tmux -L "$sock" -f /dev/null "$@"; }
trap 't kill-server 2>/dev/null || true; rm -rf "$work"' EXIT

for tool in tmux python3 freeze; do
  command -v "$tool" >/dev/null || { echo "shoot.sh: $tool not found" >&2; exit 1; }
done

go build -o "$work/pholio" "$repo/cmd/pholio"

# Copy the Vault, filling in the date tokens in file names and contents.
python3 - "$here/vault" "$work/notes" <<'EOF'
import datetime, os, re, sys
src, dst = sys.argv[1], sys.argv[2]
today = datetime.date.today()
def day(n): return today + datetime.timedelta(days=int(n))
def fill(s):
    s = re.sub(r"@D([+-]?\d+)@", lambda m: day(m.group(1)).isoformat(), s)
    s = re.sub(r"@W([+-]?\d+)@", lambda m: day(m.group(1)).strftime("%A"), s)
    return re.sub(r"@S([+-]?\d+)@", lambda m: day(m.group(1)).strftime("%Y%m%d"), s)
for root, _, files in os.walk(src):
    out = os.path.join(dst, os.path.relpath(root, src))
    os.makedirs(out, exist_ok=True)
    for f in files:
        with open(os.path.join(root, f)) as fh:
            text = fh.read()
        with open(os.path.join(out, fill(f)), "w") as fh:
            fh.write(fill(text))
EOF

# pholio's config, state and trash go to a scratch home, never the real one.
t new-session -d -s s -x 104 -y 24 \
  -e HOME="$work/home" -e XDG_CONFIG_HOME="$work/home/.config" \
  -e XDG_STATE_HOME="$work/home/.local/state" -e COLORTERM=truecolor \
  "cd '$work/notes' && '$work/pholio' '$work/notes'; sleep 60"

# wait_for TEXT: poll the screen until TEXT appears, then let it settle.
wait_for() {
  for _ in $(seq 50); do
    if t capture-pane -p -t s | grep -qF -- "$1"; then sleep 0.3; return; fi
    sleep 0.2
  done
  echo "shoot.sh: timed out waiting for '$1'. Screen:" >&2
  t capture-pane -p -t s >&2
  exit 1
}

# shot NAME: capture the screen and render docs/screenshots/NAME.png.
# freeze 0.2.2 ignores SGR 49 (default background), which lets the sidebar's
# selection colour bleed into the editor; rewrite it as a reset plus the
# foreground that was live.
shot() {
  t capture-pane -e -p -t s | python3 -c '
import re, sys
fg = None
def sub(m):
    global fg
    p = m.group(1)
    if p.startswith("38;"): fg = p
    elif p in ("39", "0", ""): fg = None
    if p == "49": return "\x1b[0m" + (f"\x1b[{fg}m" if fg else "")
    return m.group(0)
sys.stdout.write(re.sub(r"\x1b\[([0-9;]*)m", sub, sys.stdin.read()))
' > "$work/$1.ansi"
  freeze "$work/$1.ansi" --language ansi --window --padding 20 --margin 0 \
    --font.size 14 --line-height 1.25 --border.radius 8 \
    -o "$here/$1.png" </dev/null >/dev/null
  echo "wrote docs/screenshots/$1.png"
}

wait_for "NORMAL"
shot daily-note

t send-keys -t s Space t
wait_for "Upcoming"
shot task-list
