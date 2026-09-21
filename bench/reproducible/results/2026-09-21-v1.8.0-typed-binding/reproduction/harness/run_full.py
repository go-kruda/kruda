"""Run fresh capacity and all six matched-rate permutations."""
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent

def run(log_name, script, *args):
    with (ROOT / log_name).open("w") as log:
        subprocess.run([sys.executable, "-B", str(ROOT / "harness" / script), *args],
                       cwd=ROOT, stdout=log, stderr=subprocess.STDOUT, check=True)

run("capacity.log", "run_balanced.py", "measure")
run("matched-freeze.log", "run_matched_rate.py", "freeze")
run("matched-rate.log", "run_matched_rate.py", "run")
run("matched-extension-freeze.log", "run_matched_extension.py", "freeze")
run("matched-extension.log", "run_matched_extension.py")
