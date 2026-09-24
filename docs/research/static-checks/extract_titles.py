"""Write every fix commit title from the review-and-fix run logs, one per line.

Output columns, tab-separated: origin, class, title. A title often joins several
findings with "; ", and the tagger splits them.

    python3 extract_titles.py > /tmp/claude/static-checks/titles.tsv
"""
import glob
import os
import re
import sys

LOGS = os.path.expanduser("~/.melvin/config/logs/review-and-fix")
COMMIT = re.compile(r'^commit iter=\S+ sha=\S+ class=(\S+) origin=(\S+) .*title="([^"]*)')

for path in sorted(glob.glob(os.path.join(LOGS, "*.md"))):
    with open(path, errors="replace") as fh:
        for line in fh:
            m = COMMIT.match(line)
            if m:
                sys.stdout.write(f"{m.group(2)}\t{m.group(1)}\t{m.group(3)}\n")
