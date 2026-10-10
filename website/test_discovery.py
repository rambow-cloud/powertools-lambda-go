"""Exercise discovery regressions against a small copy of the strict site build."""

from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import yaml

import check_discovery as discovery


class DiscoveryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.config = yaml.safe_load((discovery.ROOT / "mkdocs.yml").read_text(encoding="utf-8"))
        self.config["nav"] = [{"Home": "index.md"}, {"Logger": "LOGGER.md"}]
        self.site = root / self.config["site_dir"]
        self.docs = root / self.config["docs_dir"]
        for name in ("index.html", "LOGGER/index.html", "404.html"):
            target = self.site / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text((discovery.ROOT / self.config["site_dir"] / name).read_text(encoding="utf-8"), encoding="utf-8")
        self.docs.mkdir()
        for name in ("index.md", "LOGGER.md"):
            (self.docs / name).write_text((discovery.ROOT / self.config["docs_dir"] / name).read_text(encoding="utf-8"), encoding="utf-8")
        self.addCleanup(patch.stopall)
        patch.object(discovery, "ROOT", root).start()
        for name, content in discovery.discovery_assets(self.config).items():
            (self.docs / name).write_text(content, encoding="utf-8")
            (self.site / name).write_text(content, encoding="utf-8")
        urls = [discovery.page_url(target, self.config["site_url"]) for target in ("index.md", "LOGGER.md")]
        self.sitemap = '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">' + ''.join(f'<url><loc>{url}</loc></url>' for url in urls) + '</urlset>'
        (self.site / "sitemap.xml").write_text(self.sitemap, encoding="utf-8")

    def replace(self, name, before, after):
        path = self.site / name
        content = path.read_text(encoding="utf-8")
        self.assertIn(before, content)
        path.write_text(content.replace(before, after), encoding="utf-8")

    def issues(self):
        errors, count = discovery.check_discovery(self.config)
        self.assertEqual(count, 2)
        return "\n".join(errors)

    def test_valid_build(self):
        self.assertEqual(self.issues(), "")

    def test_unpublished_host_in_sitemap(self):
        self.replace("sitemap.xml", self.config["site_url"], "https://preview.example/")
        self.assertIn("Sitemap must contain every canonical guide", self.issues())

    def test_accidental_noindex(self):
        self.replace("LOGGER/index.html", 'content="index, follow,', 'content="noindex, follow,')
        self.assertIn("LOGGER.md: incorrect robots", self.issues())

    def test_stale_index_after_description_change(self):
        path = self.docs / "LOGGER.md"
        content = path.read_text(encoding="utf-8").replace("Write structured JSON logs", "Create structured JSON logs")
        path.write_text(content, encoding="utf-8")
        self.assertIn("Missing or stale", self.issues())
        self.assertIn("LOGGER.md: incorrect description", self.issues())

    def test_crawler_block_in_published_asset(self):
        self.replace("robots.txt", "Allow: /", "Disallow: /")
        self.assertIn("Missing or stale", self.issues())

    def test_indexable_error_page(self):
        self.replace("404.html", "noindex, follow", "index, follow")
        self.assertIn("error page must opt out of indexing", self.issues())


if __name__ == "__main__":
    unittest.main()
