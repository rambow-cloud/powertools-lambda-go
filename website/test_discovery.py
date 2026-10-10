"""Exercise discovery regressions against a small copy of the strict site build."""

import json
from pathlib import Path
import re
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
        self.original_home = (self.site / "index.html").read_text(encoding="utf-8")
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

    def test_generic_homepage_title(self):
        path = self.site / "index.html"
        html = re.sub(r"<title>.*?</title>", "<title>Home</title>", path.read_text(encoding="utf-8"), flags=re.S)
        path.write_text(html, encoding="utf-8")
        self.assertIn("concise project-specific title", self.issues())

    def change_article_alias(self, replacement):
        path = self.site / "index.html"
        head, article = path.read_text(encoding="utf-8").split("<article", 1)
        alias = self.config["extra"]["project_aliases"][-1]
        self.assertIn(alias, article)
        path.write_text(head + "<article" + article.replace(alias, replacement), encoding="utf-8")

    def test_alias_missing_from_visible_article(self):
        self.change_article_alias("This toolkit")
        self.assertIn("visible article text", self.issues())

    def test_hidden_alias_is_not_visible_content(self):
        alias = self.config["extra"]["project_aliases"][-1]
        for element in (f'<span hidden>{alias}</span>', f'<span style="display: none">{alias}</span>', f'<script>{alias}</script>'):
            with self.subTest(element=element):
                path = self.site / "index.html"
                path.write_text(self.original_home, encoding="utf-8")
                self.change_article_alias(element)
                self.assertIn("visible article text", self.issues())

    def change_schema(self, node_type, field, value):
        path = self.site / "index.html"
        changed = False
        def replace(match):
            nonlocal changed
            schema = json.loads(match[1])
            for node in schema.get("@graph", [schema]):
                if node.get("@type") == node_type:
                    node[field] = value
                    changed = True
            return '<script type="application/ld+json">' + json.dumps(schema) + '</script>'
        html = re.sub(r'<script type="application/ld\+json">(.*?)</script>', replace, path.read_text(encoding="utf-8"), flags=re.S)
        self.assertTrue(changed)
        path.write_text(html, encoding="utf-8")

    def test_inconsistent_website_aliases(self):
        self.change_schema("WebSite", "alternateName", ["Unrelated toolkit"])
        self.assertIn("inconsistent website name or project aliases", self.issues())

    def test_article_loses_software_relationship(self):
        self.change_schema("TechArticle", "about", {"@id": self.config["site_url"] + "#website"})
        self.assertIn("article must identify the documented software", self.issues())

    def test_software_identity_points_to_wrong_repository(self):
        self.change_schema("SoftwareSourceCode", "sameAs", ["https://example.com/unrelated"])
        self.assertIn("software identity and project aliases", self.issues())


if __name__ == "__main__":
    unittest.main()
