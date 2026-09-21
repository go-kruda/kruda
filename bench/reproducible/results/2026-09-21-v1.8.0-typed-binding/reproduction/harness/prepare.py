"""Build the exact Kruda archive and freeze benchmark inputs."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parent.parent
FILES = [
    "harness/run_balanced.py", "harness/wrk_helpers.py", "harness/PROTOCOL.md",
    "sources/kruda/source.tar.gz", "sources/kruda/go.mod", "sources/kruda/commit.txt",
    "sources/fiber/main.go", "sources/fiber/go.mod", "sources/fiber/go.sum",
    "sources/actix/src/main.rs", "sources/actix/Cargo.toml", "sources/actix/Cargo.lock",
    "sources/actix/rust-version.txt", "sources/actix/build.log",
]

build = ROOT / "build" / "kruda"
if build.exists():
    shutil.rmtree(build)
build.mkdir(parents=True)
with tarfile.open(ROOT / "sources/kruda/source.tar.gz") as archive:
    archive.extractall(build, filter="data")
environment = {"GOWORK": "off", "GOTOOLCHAIN": "local"}
subprocess.run(["go", "test", "-c", "-o", str(ROOT / "bin/kruda.test"), "."],
               cwd=build, env={**__import__("os").environ, **environment}, check=True)

def sha(path):
    return hashlib.sha256((ROOT / path).read_bytes()).hexdigest()

files = {name: sha(name) for name in FILES}
binaries = {
    "kruda": {"path": "bin/kruda.test", "sha256": sha("bin/kruda.test"),
        "toolchain": {"name": "go", "version": "go1.27.1"},
        "sources": ["sources/kruda/source.tar.gz", "sources/kruda/go.mod", "sources/kruda/commit.txt"]},
    "fiber": {"path": "bin/fiber", "sha256": sha("bin/fiber"),
        "toolchain": {"name": "go", "version": "go1.27.1"},
        "sources": ["sources/fiber/main.go", "sources/fiber/go.mod", "sources/fiber/go.sum"]},
    "actix": {"path": "bin/actix", "sha256": sha("bin/actix"),
        "toolchain": {"name": "rustc", "version": "rustc 1.98.1"},
        "sources": ["sources/actix/src/main.rs", "sources/actix/Cargo.toml", "sources/actix/Cargo.lock",
                    "sources/actix/rust-version.txt", "sources/actix/build.log"]},
}
manifest = {"benchmark_lock": "/home/tiger/kruda-typed-comparison-20260912/benchmark.lock",
            "files": files, "binaries": binaries}
(ROOT / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
print(json.dumps({"kruda_sha256": binaries["kruda"]["sha256"], "manifest": str(ROOT / "manifest.json")}))
