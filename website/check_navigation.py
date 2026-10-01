"""Keep every Markdown guide in navigation when building with Zensical."""

from collections.abc import Iterator
from pathlib import Path
from urllib.parse import urlsplit

import yaml


ROOT = Path(__file__).resolve().parents[1]


def navigation_targets(value: str | list | dict) -> Iterator[str]:
    if isinstance(value, str):
        yield value
    elif isinstance(value, list):
        for item in value:
            yield from navigation_targets(item)
    elif isinstance(value, dict):
        for item in value.values():
            yield from navigation_targets(item)
    else:
        raise ValueError(f"Invalid navigation entry: {value!r}")


def check_navigation(config_path: Path) -> list[str]:
    config = yaml.safe_load(config_path.read_text(encoding="utf-8"))
    docs_dir = (config_path.parent / config["docs_dir"]).resolve()
    targets = set()
    errors = []
    for target in navigation_targets(config["nav"]):
        url = urlsplit(target)
        if url.scheme or url.netloc or url.path.startswith("/") or "\\" in url.path:
            errors.append(f"Navigation must use local Markdown paths: {target}")
            continue
        path = (docs_dir / url.path).resolve()
        if not path.is_relative_to(docs_dir):
            errors.append(f"Navigation target is outside docs_dir: {target}")
        elif path.suffix != ".md" or not path.is_file():
            errors.append(f"Navigation target is not a Markdown file: {target}")
        elif path in targets:
            errors.append(f"Duplicate navigation target: {target}")
        else:
            targets.add(path)
    for path in sorted(docs_dir.rglob("*.md")):
        if path.resolve() not in targets:
            errors.append(f"Markdown page missing from navigation: {path.relative_to(docs_dir).as_posix()}")
    return errors


if __name__ == "__main__":
    issues = check_navigation(ROOT / "mkdocs.yml")
    if issues:
        raise SystemExit("\n".join(issues))
    print("All Markdown guides are present in navigation; all targets exist.")
