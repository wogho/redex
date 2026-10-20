#!/usr/bin/env bash
# pack-license-check.sh — pre-release compliance check for Redex source packs.
#
# Usage:
#   bash scripts/pack-license-check.sh [manifest.json] [packs-dir]
#
# manifest.json defaults to ../redex-packs/manifest.json (or $REDEX_PACK_MANIFEST).
# packs-dir defaults to ./data/packs (installed packs, optional).
#
# Exit codes: 0 = all checks passed, 1 = at least one FAIL, 2 = usage/parse error.
set -u

MANIFEST="${1:-${REDEX_PACK_MANIFEST:-../redex-packs/manifest.json}}"
PACKS_DIR="${2:-${REDEX_PACKS_DIR:-./data/packs}}"

if [ ! -f "$MANIFEST" ]; then
  echo "manifest not found: $MANIFEST" >&2
  echo "usage: $0 [manifest.json] [packs-dir]" >&2
  exit 2
fi

python3 - "$MANIFEST" "$PACKS_DIR" <<'PY'
import json, os, re, sys

manifest_path, packs_dir = sys.argv[1], sys.argv[2]
try:
    doc = json.load(open(manifest_path, encoding="utf-8"))
except Exception as exc:
    print("FAIL parse manifest: %s" % exc)
    sys.exit(2)

packs = doc.get("packs", doc if isinstance(doc, list) else [])
if not isinstance(packs, list) or not packs:
    print("FAIL manifest has no packs")
    sys.exit(2)

REQUIRED = ["id", "version", "url", "sha256", "size", "license"]
FORBIDDEN = [
    ("geolite", "GeoLite2 is not redistributable"),
    ("breach", "breach/leak data must not be redistributed"),
    ("leak", "leak data must not be redistributed"),
    ("rockyou.txt", "raw leak wordlist must not be redistributed"),
]

fails, warns = [], []

def check(pack):
    pid = pack.get("id", "?")
    for key in REQUIRED:
        if not pack.get(key):
            fails.append("%s: missing %s" % (pid, key))
    sha = str(pack.get("sha256", ""))
    if sha and not re.fullmatch(r"[0-9a-f]{64}", sha.lower()):
        fails.append("%s: sha256 is not 64 hex chars" % pid)
    lic = str(pack.get("license", ""))
    if lic and not re.search(r"[A-Za-z0-9]", lic):
        fails.append("%s: empty license" % pid)
    if not pack.get("reviewed_by"):
        warns.append("%s: reviewed_by missing" % pid)
    if not pack.get("reviewed_at"):
        warns.append("%s: reviewed_at missing" % pid)
    if not pack.get("redistribution_basis"):
        warns.append("%s: redistribution_basis missing" % pid)
    url = str(pack.get("url", "")).lower()
    for needle, why in FORBIDDEN:
        if needle in url or needle in pid.lower():
            fails.append("%s: forbidden data (%s)" % (pid, why))

    # installed copy checks (optional)
    ver = str(pack.get("version", ""))
    root = os.path.join(packs_dir, pid, ver)
    if os.path.isdir(root) and pack.get("license_file_required", True):
        names = {n.lower() for n in os.listdir(root)}
        has_license = any(n.startswith(("license", "copying", "copyright")) for n in names)
        if not has_license:
            # some packs keep it one level down
            has_license = any(
                f.lower().startswith(("license", "copying", "copyright"))
                for _, _, files in os.walk(root) for f in files
            )
        if not has_license:
            fails.append("%s: LICENSE/COPYRIGHT not found in installed pack" % pid)
    elif packs_dir and os.path.isdir(packs_dir):
        pass  # not installed locally — fine for pre-release checks

for p in packs:
    if isinstance(p, dict):
        check(p)
    else:
        fails.append("manifest entry is not an object")

print("manifest: %s" % manifest_path)
print("packs:    %d" % len(packs))
for w in warns:
    print("WARN", w)
for f in fails:
    print("FAIL", f)
print("RESULT:", "PASS" if not fails else "FAIL (%d)" % len(fails))
sys.exit(0 if not fails else 1)
PY
