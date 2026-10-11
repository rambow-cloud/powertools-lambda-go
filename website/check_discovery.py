"""Generate crawler entrypoints and verify the built site's discovery contract."""

import argparse
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import struct
from urllib.parse import urljoin
from urllib.robotparser import RobotFileParser
import xml.etree.ElementTree as ET

import yaml

from check_navigation import navigation_targets


ROOT = Path(__file__).resolve().parents[1]
BOTS = (
    "Googlebot", "Googlebot-Image", "Bingbot", "DuckDuckBot", "Baiduspider", "YandexBot",
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
        "Project names: " + ", ".join(config["extra"]["project_aliases"]) + ".", "",
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


class ArticleText(HTMLParser):
    """Collect article text without metadata, scripts or explicitly hidden nodes."""

    VOID_ELEMENTS = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"}

    def __init__(self, html: str):
        super().__init__()
        self.in_article = False
        self.stack = []
        self.text = []
        self.feed(html)

    def handle_starttag(self, tag, attrs):
        if tag == "article":
            self.in_article = True
        if not self.in_article or tag in self.VOID_ELEMENTS:
            return
        attrs = dict(attrs)
        style = re.sub(r"\s+", "", attrs.get("style", "")).lower()
        hidden = (bool(self.stack and self.stack[-1][1]) or tag in {"script", "style", "template"}
                  or "hidden" in attrs or attrs.get("aria-hidden", "").lower() == "true"
                  or "display:none" in style or "visibility:hidden" in style)
        self.stack.append((tag, hidden))

    def handle_endtag(self, tag):
        if not self.in_article:
            return
        for index in range(len(self.stack) - 1, -1, -1):
            if self.stack[index][0] == tag:
                del self.stack[index:]
                break
        if tag == "article":
            self.in_article = False

    def handle_data(self, data):
        if self.in_article and self.stack and not self.stack[-1][1]:
            self.text.append(data)


def png_dimensions(data: bytes) -> tuple[int, int]:
    if len(data) < 33 or data[:16] != b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR":
        raise ValueError("invalid PNG header")
    return struct.unpack(">II", data[16:24])


def check_favicons(docs_dir: Path, site_dir: Path) -> list[str]:
    errors = []
    for name in ("favicon.png", "favicon.ico"):
        source, built = docs_dir / name, site_dir / name
        if not source.is_file() or not built.is_file():
            errors.append(f"Missing favicon asset: {name}")
            continue
        data = built.read_bytes()
        if data != source.read_bytes():
            errors.append(f"Stale favicon asset: {name}")
        try:
            if name.endswith(".png"):
                if png_dimensions(data) != (96, 96):
                    raise ValueError("PNG must be 96x96")
            else:
                reserved, kind, count = struct.unpack_from("<HHH", data)
                if (reserved, kind, count) != (0, 1, 4):
                    raise ValueError("ICO must contain four icon frames")
                sizes = set()
                previous_end = 6 + 16 * count
                for index in range(count):
                    width, height, colors, entry_reserved, planes, bits, length, offset = struct.unpack_from("<BBBBHHII", data, 6 + 16 * index)
                    size = (width or 256, height or 256)
                    if (colors or entry_reserved or planes not in (0, 1) or bits != 32
                            or offset < previous_end or offset + length > len(data)
                            or png_dimensions(data[offset:offset + length]) != size):
                        raise ValueError("invalid ICO frame")
                    sizes.add(size)
                    previous_end = offset + length
                if sizes != {(16, 16), (32, 32), (48, 48), (256, 256)}:
                    raise ValueError("ICO must contain 16, 32, 48 and 256px square frames")
        except (ValueError, struct.error) as error:
            errors.append(f"Invalid favicon asset {name}: {error}")
    return errors


def check_discovery(config: dict) -> tuple[list[str], int]:
    docs_dir, site_dir = ROOT / config["docs_dir"], ROOT / config["site_dir"]
    site_url = config["site_url"]
    aliases = config["extra"]["project_aliases"]
    errors = check_favicons(docs_dir, site_dir)
    if config["theme"].get("favicon") != "favicon.ico":
        errors.append("Theme must declare the dedicated favicon.ico.")
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
        icons = [urljoin(url, href) for href in head.links.get("icon", [])]
        if icons != [site_url + "favicon.ico", site_url + "favicon.png"]:
            errors.append(f"{target}: missing or incorrect dedicated favicon links")
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
        websites = [node for node in nodes if node.get("@type") == "WebSite"]
        if (len(websites) != 1 or websites[0].get("name") != config["site_name"]
                or websites[0].get("url") != site_url or websites[0].get("alternateName") != aliases):
            errors.append(f"{target}: inconsistent website name or project aliases")
        articles = [node for node in nodes if node.get("@type") == "TechArticle"]
        if len(articles) != 1 or articles[0].get("url") != url or articles[0].get("description") != description:
            errors.append(f"{target}: missing or inconsistent TechArticle structured data")
        if len(articles) == 1 and articles[0].get("about") != {"@id": site_url + "#software"}:
            errors.append(f"{target}: article must identify the documented software")
        if target == "index.md":
            expected_title = metadata.get("title", config["site_name"])
            if head.title.strip() != expected_title or aliases[0].lower() not in expected_title.lower():
                errors.append("Homepage must have one concise project-specific title without a repeated site-name suffix.")
            visible_text = re.sub(r"\s+", " ", " ".join(ArticleText(html).text)).lower()
            if any(alias.lower() not in visible_text for alias in aliases):
                errors.append("Homepage project aliases must appear in visible article text.")
            software = [node for node in nodes if node.get("@type") == "SoftwareSourceCode"]
            if len(software) != 1 or software[0].get("codeRepository") != config["repo_url"]:
                errors.append("Homepage must identify the software source repository.")
            if (len(software) != 1 or software[0].get("alternateName") != aliases
                    or software[0].get("url") != site_url or software[0].get("sameAs") != [config["repo_url"]]):
                errors.append("Homepage software identity and project aliases must match the canonical site and repository.")
        if any(not robots.can_fetch(bot, url) for bot in BOTS):
            errors.append(f"{target}: crawler access denied")
        if '<article' not in html:
            errors.append(f"{target}: missing server-rendered article")
    error_head = PageHead((site_dir / "404.html").read_text(encoding="utf-8"))
    published_robots = RobotFileParser()
    published_robots.parse((site_dir / "robots.txt").read_text(encoding="utf-8").splitlines())
    for name in ("favicon.ico", "favicon.png"):
        if not published_robots.can_fetch("Googlebot-Image", site_url + name):
            errors.append(f"Googlebot-Image cannot crawl {name}")
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
