"""Generate notes at a frozen source commit without preparing or publishing modules."""

import json

import release
from release_notes import render_unified_notes
from release_prepare import build_notes_plan


MARKER = "<!-- powertools-release-notes-preview -->"


def preview(args, api):
    release.version_key(args.base)
    target = release.git("rev-parse", "--verify", "--end-of-options", args.target + "^{commit}")
    baseline = release.commit_of_tag(args.base)
    if not release.is_ancestor(baseline, target):
        raise ValueError("The selected baseline must be an ancestor of the source commit.")
    releases = list(api.pages("releases"))
    if not any(item["tag_name"] == args.base and not item["draft"] for item in releases):
        raise ValueError("The baseline must be a published project Release.")
    data = json.loads(release.git("show", target + ":tools/modules.json"))
    if data["base"] != "github.com/" + release.REPOSITORY:
        raise ValueError("Unexpected module namespace at the selected source commit.")
    modules = {item["directory"]: item for item in data["modules"]}
    # Restrict history boundaries to the chosen baseline, even if later versions exist.
    prior = [item for item in releases if release.VERSION.fullmatch(item["tag_name"].rsplit("/", 1)[-1])
             and release.version_key(item["tag_name"].rsplit("/", 1)[-1]) <= release.version_key(args.base)]
    plan = build_notes_plan(api, modules, target, prior, {}, args.bump, [])
    plan["preview"] = True
    version = plan["release_version"]
    notes = MARKER + "\n\n" + render_unified_notes(release.REPOSITORY, plan)
    output = release.ROOT / "dist/releases/previews" / version / target[:12]
    release.write_json(output / "preview.json", plan)
    (output / "notes.md").write_text(notes, encoding="utf-8", newline="\n")
    print(f"Notes preview: {output / 'notes.md'}")
    print(f"Baseline: {args.base} ({baseline}); source: {target}; candidate: {version}")
    if args.draft:
        # This namespace cannot reserve a root/nested Go module version.
        tag = "notes-preview/" + version
        matches = [item for item in releases if item["tag_name"] == tag]
        if len(matches) > 1 or any(not item["draft"] or MARKER not in (item.get("body") or "") for item in matches):
            raise ValueError("Refusing to replace an unrelated or published Release.")
        if release.remote_tag(api, tag):
            raise ValueError("A notes preview must not target an existing Git tag.")
        payload = {"tag_name": tag, "target_commitish": target, "name": version + " — Release notes preview",
                   "body": notes, "draft": True, "prerelease": False, "make_latest": "false"}
        if matches:
            release.write_json(output / "previous-draft.json", matches[0])
            result = api.repo(f"releases/{matches[0]['id']}", method="PATCH", data=payload)
        else:
            result = api.repo("releases", method="POST", data=payload)
        print("GitHub draft: " + result["html_url"])
    return plan
