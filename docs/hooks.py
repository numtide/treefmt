import subprocess
from pathlib import Path

def on_pre_build(**kwargs):
    with Path("./snippets/usage.txt").open("w") as f:
        subprocess.run(
            ["treefmt", "--help"],
            stdout=f,
            check=True,
        )
