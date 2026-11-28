"""Fail when gosec reports a finding on a Go line changed since a base commit.

Inherited code carries findings that are triaged separately. This gate stops
new ones: it runs gosec on the packages that hold changed Go files and fails
only for findings whose lines overlap the lines added or changed since the
base. A finding that is a false positive is marked in the source with
`#nosec <rule> -- <reason>`, which gosec honours.

Usage: check_gosec_changed.py <base-ref> [gosec-binary]
"""
import json
import os
import re
import subprocess
import sys
import tempfile

HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")


def parse_diff(text):
    """Map each changed path to the set of its added or changed line numbers."""
    changed = {}
    current = None
    for line in text.splitlines():
        if line.startswith("+++ "):
            path = line[4:]
            current = None if path == "/dev/null" else path[2:]
        elif line.startswith("@@") and current:
            match = HUNK.match(line)
            if not match:
                continue
            start = int(match.group(1))
            count = 1 if match.group(2) is None else int(match.group(2))
            if count:
                changed.setdefault(current, set()).update(range(start, start + count))
    return changed


def issue_lines(value):
    """Return the line numbers of a gosec line field: "12" or "12-15"."""
    first, _, last = str(value).partition("-")
    first = int(first)
    return range(first, (int(last) if last else first) + 1)


def new_findings(issues, changed, root):
    """Return the issues that fall on changed lines, as (path, line, text)."""
    found = []
    for issue in issues:
        path = os.path.relpath(issue["file"], root)
        lines = changed.get(path)
        if not lines:
            continue
        if any(n in lines for n in issue_lines(issue["line"])):
            found.append((path, issue["line"], "%s %s: %s" % (issue["rule_id"], issue["severity"], issue["details"])))
    return sorted(found)


def main(argv):
    if len(argv) < 2:
        print("usage: check_gosec_changed.py <base-ref> [gosec-binary]", file=sys.stderr)
        return 2
    base, gosec = argv[1], (argv[2] if len(argv) > 2 else "gosec")
    root = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()
    diff = subprocess.run(
        ["git", "diff", "-U0", "--no-color", "--diff-filter=AMR", base + "...HEAD", "--", "*.go"],
        capture_output=True, text=True, check=True, cwd=root).stdout
    changed = {p: l for p, l in parse_diff(diff).items() if not p.endswith("_test.go")}
    if not changed:
        print("gosec: no changed Go files")
        return 0
    packages = sorted({"./" + (os.path.dirname(p) or ".") for p in changed})
    with tempfile.TemporaryDirectory() as tmp:
        out = os.path.join(tmp, "gosec.json")
        # gosec exits non-zero whenever it finds anything, including findings on
        # inherited lines, so its status is not the result; the report is.
        subprocess.run([gosec, "-quiet", "-fmt", "json", "-out", out] + packages, cwd=root)
        try:
            with open(out) as handle:
                report = json.load(handle)
        except (OSError, ValueError):
            print("gosec: no report was written", file=sys.stderr)
            return 2
    errors = report.get("Golang errors") or {}
    if errors:
        print("gosec: could not load %d file(s); fix the build first" % len(errors), file=sys.stderr)
        return 2
    found = new_findings(report.get("Issues") or [], changed, root)
    for path, line, text in found:
        print("%s:%s: %s" % (path, line, text))
    if found:
        print("gosec: %d finding(s) on changed lines. Fix them, or mark a confirmed false positive with `#nosec <rule> -- <reason>`." % len(found))
        return 1
    print("gosec: no findings on changed lines in %d package(s)" % len(packages))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
