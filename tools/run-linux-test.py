"""Execute a locally cross-compiled Go binary in the pinned Linux Lambda image."""
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
TEMPORARY = ROOT / "dist/local/go-tmp"
IMAGE = "public.ecr.aws/lambda/provided@sha256:0b17e5c778aef6ed7f61cbfa5dd416e17540e127846250646fceb712e0e7be5f"

def mapped(value):
    path = Path(value)
    if not path.is_absolute():
        return value
    path = path.resolve()
    for source, destination in ((TEMPORARY, "/go-tmp"), (ROOT, "/workspace")):
        if path.is_relative_to(source):
            return destination + "/" + path.relative_to(source).as_posix()
    raise ValueError(f"Binary or test artifact is outside the workspace: {path}")

def main():
    if len(sys.argv) < 2:
        raise SystemExit("Usage: run-linux-test.py BINARY [ARGS...]")
    binary = mapped(str(Path(sys.argv[1]).resolve()))
    args = []
    for argument in sys.argv[2:]:
        key, separator, value = argument.partition("=")
        args.append(key + separator + mapped(value) if separator else mapped(argument))
    command = ["docker", "run", "--rm", "--interactive", "--network", "none",
               "--platform", "linux/amd64", "--workdir", mapped(str(Path.cwd())),
               "--mount", f"type=bind,source={ROOT},target=/workspace,readonly",
               "--mount", f"type=bind,source={TEMPORARY},target=/go-tmp",
               "--env", "CGO_ENABLED=0", "--env", "AWS_EC2_METADATA_DISABLED=true",
               "--entrypoint", binary, IMAGE, *args]
    raise SystemExit(subprocess.run(command).returncode)

if __name__ == "__main__":
    main()
