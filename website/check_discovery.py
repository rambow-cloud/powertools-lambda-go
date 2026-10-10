"""Generate crawler entrypoints and verify the built site's discovery contract."""

import argparse
from html.parser import HTMLParser
import json
from pathlib import Path
import re
from urllib.parse import urljoin
from urllib.robotparser import RobotFileParser
import xml.etree.ElementTree as ET

import yaml

from check_navigation import navigation_targets


ROOT = Path(__file__).resolve().parents[1]
BOTS = (
    "Googlebot", "Bingbot", "DuckDuckBot", "Baiduspider", "YandexBot",
    "OAI-SearchBot", "ChatGPT-User", "GPTBot", "Claude-SearchBot",
    "Claude-User", "ClaudeBot", "PerplexityBot", "Perplexity-User",
    "Amazonbot", "Applebot", "Google-Extended", "CCBot", "UnknownBot",
)


def page_metadata(path: Path) -> tuple[dict, str]:
    source = path.read_text(encoding="utf-8")
    metadata = {}
    if source.startswith("---\n"):
        header, source = source[4:].split("\n---\n", 1)
        metadata = yaml.safe_load(header) or {}
    heading = re.search(r"^# (.+)$", source, re.M)
    if not heading:
        raise ValueError(f"Missing page heading: {path}")
    return metadata, heading[1]


def page_url(target: str, site_url: str) -> str:
    path = Path(target).with_suffix("").as_posix()
    if path == "index":
        path = ""
    elif path.endswith("/index"):
        path = path[:-5]
    else:
        path += "/"
    return urljoin(site_url, path)


def discovery_assets(config: dict) -> dict[str, str]:
    site_url = config["site_url"]
    if not site_url.startswith("https://") or not site_url.endswith("/"):
        raise ValueError("site_url must be an absolute HTTPS URL ending in /.")
    if not config.get("use_directory_urls", True):
        raise ValueError("Discovery generation requires directory URLs.")
    docs_dir = ROOT / config["docs_dir"]
    lines = [
        f"# {config['site_name']}", "",
        f"> {config['site_description']}", "",
        "This is an independent community project, not an official AWS distribution.",
        "The English documentation follows main; Release notes identify published versions.",
        "Use Compatibility and the TypeScript feature comparison for supported boundaries.",
        "Target AWS Lambda provided.al2023 with CGO disabled on Linux amd64 or arm64.",
        "Use OpenTelemetry and a collector's awsxray exporter for AWS X-Ray.",
        "The legacy tracer/xray SDK adapter is deprecated and frozen.", "",
        "The Optional section contains maintainer procedures, implementation plans and dated evidence.",
        "Plans describe intended work; historical evidence establishes only its stated scope.",
        "These pages do not override current usage guides or published release contracts.", "",
        f"- [Source repository]({config['repo_url']})", "",
    ]
    optional = []
    for entry in config["nav"]:
        label, value = next(iter(entry.items()))
        targets = list(navigation_targets(value))
        section = [f"## {label}", ""]
        for target in targets:
            metadata, title = page_metadata(docs_dir / target)
            description = metadata.get("description")
            suffix = f": {description}" if description else ""
            section.append(f"- [{title}]({page_url(target, site_url)}){suffix}")
        section.append("")
        if label == "Development":
            optional.extend(section[2:])
        else:
            lines.extend(section)
    lines.extend(["## Optional", "", *optional])
    return {
        "robots.txt": f"User-agent: *\nAllow: /\n\nSitemap: {site_url}sitemap.xml\n",
        "llms.txt": "\n".join(lines).rstrip() + "\n",
    }


class PageHead(HTMLParser):
    """Read metadata from static HTML without executing browser scripts."""

    def __init__(self, html: str):
        super().__init__()
        self.meta = {}
        self.links = {}
        self.schemas = []
        self.title = ""
        self.in_title = False
        self.in_schema = False
        self.schema_parts = []
        self.feed(html)

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "meta":
            key = attrs.get("name", attrs.get("property", ""))
            self.meta.setdefault(key, []).append(attrs.get("content", ""))
        elif tag == "link":
            self.links.setdefault(attrs.get("rel", ""), []).append(attrs.get("href", ""))
        elif tag == "title":
            self.in_title = True
        elif tag == "script" and attrs.get("type") == "application/ld+json":
            self.in_schema = True
            self.schema_parts = []

    def handle_data(self, data):
        if self.in_title:
            self.title += data
        if self.in_schema:
            self.schema_parts.append(data)

    def handle_endtag(self, tag):
        if tag == "title":
            self.in_title = False
        elif tag == "script" and self.in_schema:
            self.schemas.append(json.loads("".join(self.schema_parts)))
            self.in_schema = False


def check_discovery(config: dict) -> tuple[list[str], int]:
    docs_dir, site_dir = ROOT / config["docs_dir"], ROOT / config["site_dir"]
    site_url = config["site_url"]
    errors = []
    for filename, expected in discovery_assets(config).items():
        for directory in (docs_dir, site_dir):
            path = directory / filename
            if not path.is_file() or path.read_text(encoding="utf-8") != expected:
                errors.append(f"Missing or stale {path.relative_to(ROOT)}; run --write and rebuild.")
    robots = RobotFileParser()
    robots.parse(discovery_assets(config)["robots.txt"].splitlines())
    targets = list(navigation_targets(config["nav"]))
    urls = {page_url(target, site_url) for target in targets}
    sitemap = ET.parse(site_dir / "sitemap.xml")
    locations = [node.text for node in sitemap.findall(".//{*}loc")]
    if set(locations) != urls or len(locations) != len(urls):
        errors.append("Sitemap must contain every canonical guide exactly once.")
    for target in targets:
        url = page_url(target, site_url)
        relative = url.removeprefix(site_url)
        path = site_dir / relative / "index.html"
        if not path.is_file():
            errors.append(f"Missing static HTML: {path.relative_to(ROOT)}")
            continue
        html = path.read_text(encoding="utf-8")
        head = PageHead(html)
        metadata, _ = page_metadata(docs_dir / target)
        description = metadata.get("description", config["site_description"])
        if not head.title.strip() or head.links.get("canonical") != [url]:
            errors.append(f"{target}: missing title or incorrect canonical URL")
        for name, expected in {
            "description": description, "og:description": description,
            "og:url": url, "twitter:description": description,
            "robots": "index, follow, max-snippet:-1, max-image-preview:large, max-video-preview:-1",
        }.items():
            if head.meta.get(name) != [expected]:
                errors.append(f"{target}: incorrect {name}")
        if head.links.get("describedby") != [site_url + "llms.txt"]:
            errors.append(f"{target}: missing AI documentation index link")
        nodes = [node for schema in head.schemas for node in schema.get("@graph", [schema])]
        articles = [node for node in nodes if node.get("@type") == "TechArticle"]
        if len(articles) != 1 or articles[0].get("url") != url or articles[0].get("description") != description:
            errors.append(f"{target}: missing or inconsistent TechArticle structured data")
        if target == "index.md":
            software = [node for node in nodes if node.get("@type") == "SoftwareSourceCode"]
            if len(software) != 1 or software[0].get("codeRepository") != config["repo_url"]:
                errors.append("Homepage must identify the software source repository.")
        if any(not robots.can_fetch(bot, url) for bot in BOTS):
            errors.append(f"{target}: crawler access denied")
        if '<article' not in html:
            errors.append(f"{target}: missing server-rendered article")
    error_head = PageHead((site_dir / "404.html").read_text(encoding="utf-8"))
    if error_head.meta.get("robots") != ["noindex, follow"]:
        errors.append("The error page must opt out of indexing.")
    return errors, len(targets)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true", help="Regenerate tracked robots.txt and llms.txt before building.")
    args = parser.parse_args()
    config = yaml.safe_load((ROOT / "mkdocs.yml").read_text(encoding="utf-8"))
    if args.write:
        for name, content in discovery_assets(config).items():
            (ROOT / config["docs_dir"] / name).write_text(content, encoding="utf-8", newline="\n")
        print("Generated docs/robots.txt and docs/llms.txt from canonical configuration and navigation.")
        return
    errors, count = check_discovery(config)
    if errors:
        raise SystemExit("\n".join(errors))
    print(f"Verified {count} static guides, canonical URLs, sitemap, metadata, JSON-LD and {len(BOTS)} crawler policies.")


if __name__ == "__main__":
    main()
