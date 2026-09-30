"""Build the official Lambda collector with the AWS X-Ray exporter enabled."""
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "dist" / "integration"
SOURCE = OUT / "collector-source"
URL = "https://codeload.github.com/open-telemetry/opentelemetry-lambda/tar.gz/refs/tags/layer-collector/0.23.0"
cached = OUT / "collector-source.tar.gz"
if not cached.exists():
    print("Downloading pinned official collector sources", flush=True)
    with urllib.request.urlopen(URL, timeout=60) as response:
        cached.write_bytes(response.read())
data = cached.read_bytes()
SOURCE.mkdir(parents=True, exist_ok=True)
with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
    for member in archive.getmembers():
        parts = Path(member.name).parts
        if len(parts) < 3 or parts[1] != "collector" or not member.isfile():
            continue
        target = (SOURCE / Path(*parts[2:])).resolve()
        if not target.is_relative_to(SOURCE.resolve()):
            raise RuntimeError("Invalid archive path")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(archive.extractfile(member).read())
factory = SOURCE / "lambdacomponents" / "default.go"
factory.write_bytes((ROOT / "integration" / "collector_components.go.txt").read_bytes())
for directory in ("go-cache", "go-tmp"):
    (OUT / directory).mkdir(exist_ok=True)
env = {**os.environ, "CGO_ENABLED": "0", "GOCACHE": str(OUT / "go-cache"), "GOTMPDIR": str(OUT / "go-tmp")}
subprocess.run(["go", "get", "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter@v0.157.0"], cwd=SOURCE / "lambdacomponents", env=env, check=True)
subprocess.run(["go", "mod", "tidy"], cwd=SOURCE, env=env, check=True)
evidence = {"source_url": URL, "source_sha256": hashlib.sha256(data).hexdigest(), "modification": "Use a trace-only component factory: OTLP receiver and awsxrayexporter v0.157.0", "cgo_enabled": False, "hashes": {}}
for arch in ("amd64", "arm64"):
    binary = OUT / f"collector-custom-{arch}"
    subprocess.run(["go", "build", "-trimpath", "-ldflags=-s -w -X main.Version=v0.157.0", "-o", str(binary), "."], cwd=SOURCE, env={**env, "GOOS": "linux", "GOARCH": arch}, check=True)
    with zipfile.ZipFile(OUT / f"collector-custom-{arch}.zip", "w", zipfile.ZIP_DEFLATED) as archive:
        entry = zipfile.ZipInfo("extensions/collector")
        entry.create_system = 3
        entry.external_attr = 0o100755 << 16
        entry.compress_type = zipfile.ZIP_DEFLATED
        archive.writestr(entry, binary.read_bytes())
    evidence["hashes"][arch] = hashlib.sha256(binary.read_bytes()).hexdigest()
    print(f"Built custom collector {arch} with CGO_ENABLED=0", flush=True)
(OUT / "collector-build.json").write_text(json.dumps(evidence, indent=2) + "\n", encoding="utf-8")
