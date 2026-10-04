"""Automate reviewable release preparation without publishing any module tags."""

from copy import deepcopy
import hashlib
import json
import os
from pathlib import PurePosixPath
import re
import sys
import time
from urllib.parse import quote

import release
from release_notes import parse_notes, render_notes, section, validate_direct_note


def maintained(module):
    return module["public"] and module.get("status") != "deprecated-frozen"


def scope_modules(modules, selected=None, all_modules=False):
    if all_modules and selected:
        raise ValueError("Choose --all or --module, not both.")
    result = list(modules) if all_modules else list(selected or [])
    if all_modules:
        result = [directory for directory in result if maintained(modules[directory])]
    if not result or len(result) != len(set(result)):
        raise ValueError("Select unique module directories or --all.")
    for directory in result:
        if directory not in modules or not maintained(modules[directory]):
            raise ValueError(f"Not a maintained public release module: {directory}")
    return result


def affected_modules(paths, modules):
    """Assign each path to its deepest module; repository tooling stays out of Commons."""
    affected = set()
    nested = sorted((directory for directory in modules if directory != "."), key=len, reverse=True)
    for filename in paths:
        path = PurePosixPath(filename)
        if path.is_absolute() or ".." in path.parts:
            raise ValueError("Unexpected historical file path.")
        match = next((directory for directory in nested if filename.startswith(directory + "/")), None)
        if match:
            affected.add(match)
        elif filename.startswith("commons/") or ("/" not in filename and filename.endswith(".go")) or filename in {"go.mod", "go.sum"}:
            affected.add(".")
        else:
            affected.add("repository")
    return sorted(affected or {"repository"})


def title_kind(title):
    if re.match(r"^[a-z]+(?:\([^\n)]*\))?!:", title):
        return "breaking"
    prefix = re.match(r"^([a-z]+)(?:\([^\n)]*\))?:", title)
    return {"feat": "feature", "fix": "fix", "docs": "documentation"}.get(prefix[1] if prefix else "", "maintenance")


def one_line(text):
    return " ".join(text.split()).replace("|", "/")[:1200]


class History:
    def __init__(self, api, modules, target, overrides):
        self.api, self.modules, self.target, self.overrides = api, modules, target, overrides
        self.commits, self.ranges, self.pr_notes, self.direct = {}, {}, {}, {}

    def notes(self, previous):
        if previous not in self.ranges:
            self.ranges[previous] = release.merged_prs(self.api, release.commit_of_tag(previous) if previous else None, self.target, self.commits)
        prs, uncovered = self.ranges[previous]
        entries, generated = [], []
        for pr in prs:
            number = pr["number"]
            if number not in self.pr_notes:
                replacement = self.overrides.get("pull_requests", {}).get(str(number))
                body = pr.get("body") or ""
                if replacement is not None:
                    body = "## Release notes\n\n" + replacement
                elif not section(body, "Release notes"):
                    # File-count coverage is available only on the full PR response.
                    pr = self.api.repo(f"pulls/{number}")
                    body = pr.get("body") or ""
                if section(body, "Release notes"):
                    parsed, provenance = parse_notes(body, self.modules), None
                else:
                    if pr.get("changed_files", 0) > 3000:
                        raise ValueError(f"PR #{number} exceeds GitHub's file listing limit; supply an override.")
                    files = list(self.api.pages(f"pulls/{number}/files"))
                    if pr.get("changed_files") and len(files) != pr["changed_files"]:
                        raise ValueError(f"Incomplete file coverage for PR #{number}; supply an override.")
                    paths = [filename for file in files for filename in (file["filename"], file.get("previous_filename")) if filename]
                    parsed = [{"module": directory, "type": title_kind(pr["title"]), "description": one_line(pr["title"])} for directory in affected_modules(paths, self.modules)]
                    provenance = {"pr": number, "source": "PR title and changed module paths", "modules": [entry["module"] for entry in parsed]}
                self.pr_notes[number] = (parsed, provenance)
            parsed, provenance = self.pr_notes[number]
            entries.extend({**entry, "pr": number} for entry in parsed)
            if provenance:
                generated.append(provenance)
        direct = {}
        for sha in uncovered:
            if sha not in self.direct:
                note = self.overrides.get("untracked_commits", {}).get(sha)
                if note is None:
                    paths = release.git("diff-tree", "--root", "--no-commit-id", "--name-only", "-r", "-m", sha).splitlines()
                    note = {"modules": affected_modules(paths, self.modules), "description": one_line(release.git("show", "-s", "--format=%s", sha))}
                validate_direct_note(note, self.modules)
                self.direct[sha] = note
            direct[sha] = self.direct[sha]
        return entries, [pr["number"] for pr in prs], direct, generated


def next_version(current, previous, bump, entries, reserved=()):
    """First release keeps the configured version; later requests always advance."""
    release.version_key(current)
    baseline = previous.rsplit("/", 1)[-1] if previous else None
    if baseline is None:
        version = current
    else:
        major, minor, patch, *_ = release.version_key(baseline)
        if bump == "auto":
            types = {entry["type"] for entry in entries}
            bump = "major" if "breaking" in types and major else "minor" if types & {"feature", "breaking"} else "patch"
        if bump == "major":
            major, minor, patch = major + 1, 0, 0
        elif bump == "minor":
            minor, patch = minor + 1, 0
        else:
            patch += 1
        version = f"v{major}.{minor}.{patch}"
        if release.version_key(current) > release.version_key(version):
            version = current
    release.version_key(version)
    # Reserved tags include drafts and tags without Releases, across all branches.
    for existing in sorted(set(reserved), key=release.version_key):
        if release.version_key(existing) >= release.version_key(version):
            major, minor, patch, *_ = release.version_key(existing)
            version = f"v{major}.{minor}.{patch}" if "-" in existing else f"v{major}.{minor}.{patch + 1}"
            if version == existing:
                raise ValueError("Automatic preparation cannot reuse a reserved version.")
    release.version_key(version)
    return version


def go_environment():
    env = os.environ.copy()
    env.update(CGO_ENABLED="0", GOWORK="off")
    return env


def module_requirements(modules):
    result = {}
    for directory in modules:
        data = json.loads(release.run("go", "mod", "edit", "-json", cwd=release.ROOT / directory, env=go_environment()))
        expected = "github.com/" + release.REPOSITORY + ("" if directory == "." else "/" + directory)
        if data["Module"]["Path"] != expected or data.get("Replace"):
            raise ValueError(f"Unexpected module path or local replacements: {directory}")
        result[directory] = data.get("Require") or []
    return result


def select_dependencies(selected, modules, requirements, available):
    paths = {"github.com/" + release.REPOSITORY + ("" if directory == "." else "/" + directory): directory for directory in modules}
    result = list(selected)
    for directory in result:
        for requirement in requirements[directory]:
            dependency = paths.get(requirement["Path"])
            if dependency is None:
                if requirement["Path"].startswith("github.com/" + release.REPOSITORY + "/"):
                    raise ValueError(f"Unknown internal dependency: {requirement['Path']}")
                continue
            if not maintained(modules[dependency]):
                raise ValueError(f"Unsupported release dependency: {dependency}")
            if dependency not in result and not available(release.tag_name(dependency, requirement["Version"])):
                result.append(dependency)
    return result


def synchronize_metadata(modules, selected, requirements):
    """Keep all workspace consumers coherent without adding them to publication scope."""
    versions = {"github.com/" + release.REPOSITORY + ("" if directory == "." else "/" + directory): item["version"] for directory, item in selected.items()}
    updates = {}
    env = go_environment()
    for directory, requirements_list in requirements.items():
        changed = []
        edits = []
        for requirement in requirements_list:
            path = requirement["Path"]
            if path in versions and requirement["Version"] != versions[path]:
                changed.append({"path": path, "from": requirement["Version"], "to": versions[path]})
                requirement["Version"] = versions[path]
                edits.append(f"-require={path}@{versions[path]}")
        if edits:
            release.run("go", "mod", "edit", *edits, cwd=release.ROOT / directory, env=env)
            updates[directory] = changed
    manifest_path = release.ROOT / "tools/modules.json"
    data = json.loads(manifest_path.read_text(encoding="utf-8"))
    for module in data["modules"]:
        if module["directory"] in selected:
            module["version"] = selected[module["directory"]]["version"]
    release.write_json(manifest_path, data)
    workspace = json.loads(release.run("go", "work", "edit", "-json", str(release.ROOT / "go.work"), cwd=release.ROOT, env=env))
    if any(replacement["Old"]["Path"] not in {"github.com/" + release.REPOSITORY + ("" if directory == "." else "/" + directory) for directory in modules} for replacement in workspace.get("Replace") or []):
        raise ValueError("Release preparation will not rewrite external workspace replacements.")
    edits = ["-dropreplace=" + entry["Old"]["Path"] + ("@" + entry["Old"]["Version"] if entry["Old"].get("Version") else "") for entry in workspace.get("Replace") or []]
    mappings = {(requirement["Path"], requirement["Version"]) for values in requirements.values() for requirement in values if requirement["Path"] in versions}
    mappings.update(("github.com/" + release.REPOSITORY + ("" if directory == "." else "/" + directory), selected.get(directory, modules[directory])["version"]) for directory in modules)
    paths = {"github.com/" + release.REPOSITORY + ("" if directory == "." else "/" + directory): directory for directory in modules}
    # Include unchanged internal requirements as well as the new manifest versions.
    mappings.update((requirement["Path"], requirement["Version"]) for values in requirements.values() for requirement in values if requirement["Path"] in paths)
    edits.extend(f"-replace={path}@{version}={'.' if paths[path] == '.' else './' + paths[path]}" for path, version in sorted(mappings))
    release.run("go", "work", "edit", *edits, str(release.ROOT / "go.work"), cwd=release.ROOT, env=env)
    release.run(sys.executable, str(release.ROOT / "tools/modules.py"), "tidy", cwd=release.ROOT, env=env)
    return updates


def preparation_body(plan, name):
    rows = "\n".join(f"| `{item['directory']}` | `{item['version']}` | `{item['previous_tag'] or 'initial release'}` |" for item in plan["modules"])
    inferred = sorted({record["pr"] for item in plan["modules"] for record in item.get("generated_history", [])})
    notes = ["- repository | maintenance | Prepare reviewed module versions and accumulated release notes."]
    selected = {item["directory"] for item in plan["modules"]}
    for directory in plan.get("metadata_updates", {}):
        if directory not in selected:
            notes.append(f"- {directory} | maintenance | Synchronize internal dependencies with this release batch.")
    automatic = plan.get("auto_publish", False)
    history = ", ".join(f"#{number}" for number in inferred) or "None"
    return f"""## Issue

Refs #{plan['issue']}

## Summary

Prepare `releases/{name}.json` from `{plan['source_sha']}`. Versions, internal requirements, workspace mappings, dependency sums, and accumulated module notes were generated automatically.

| Module | Target | Previous release |
|---|---|---|
{rows}

Requested modules: {', '.join('`' + directory + '`' for directory in plan['requested_modules'])}. Automatically included dependencies: {', '.join('`' + directory + '`' for directory in plan['included_dependencies']) or 'None'}.

Historical notes inferred from PR titles and changed paths: {history}. Review these summaries, initial scope statements, and direct-commit acknowledgements in the plan.

## Testing

Internal dependency metadata was tidied with CGO disabled and no module-file replacements. Native contribution, module, and documentation PR checks are activated for this exact head; review their results before merging. If GitHub requires maintainer authorization, select **Approve workflows to run** in this PR.

## Risks and compatibility

Automatic publication after merge and successful main checks: **{'enabled' if automatic else 'disabled'}**. {'Merging this preparation PR authorizes the frozen release batch; the workflow publishes with GoReleaser and closes the tracking issue.' if automatic else 'Use Publish Go modules with this PR number after merging.'} Squash or merge onto the recorded source SHA. If main advances, rerun preparation against main and replace this stale PR. Tags are never rewritten.

## Release notes

{chr(10).join(notes)}
"""


def start_pr_checks(pr, api, resume=False, wait_seconds=60):
    """Native PR events satisfy rulesets; workflow_dispatch job checks do not."""
    expected = {".github/workflows/" + filename for filename in ("contribution.yml", "ci.yml", "docs.yml")}
    deadline = time.monotonic() + wait_seconds
    while True:
        latest, page = {}, 1
        while True:
            runs = api.repo(f"actions/runs?event=pull_request&head_sha={pr['head']['sha']}&per_page=100&page={page}")["workflow_runs"]
            for run in runs:
                path = run["path"].split("@", 1)[0]
                if (path in expected and run["event"] == "pull_request" and run["head_sha"] == pr["head"]["sha"]
                        and run["head_repository"]["full_name"] == release.REPOSITORY
                        and any(item["number"] == pr["number"] for item in run["pull_requests"])
                        and run["id"] > latest.get(path, {}).get("id", -1)):
                    latest[path] = run
            if len(runs) < 100:
                break
            page += 1
        if set(latest) == expected:
            break
        if time.monotonic() >= deadline:
            raise ValueError("Native PR checks have not appeared yet. Open the preparation PR, approve workflows if requested, then rerun preparation to resume; dispatch checks cannot replace required PR checks.")
        time.sleep(2)
    for run in latest.values():
        if run["conclusion"] == "action_required":
            try:
                api.repo(f"actions/runs/{run['id']}/approve", method="POST", data={})
            except RuntimeError as error:
                if "HTTP 403" not in str(error):
                    raise
                raise ValueError("GitHub requires maintainer authorization: open the preparation PR and select Approve workflows to run. Existing native checks were retained; no substitute checks were dispatched.") from None
        elif resume and run["status"] == "completed" and run["conclusion"] in {"failure", "cancelled", "timed_out"}:
            api.repo(f"actions/runs/{run['id']}/rerun", method="POST", data={})


def open_preparation(plan, name, branch, api):
    pr = api.repo("pulls", method="POST", data={"title": "chore(release): prepare " + name, "head": branch, "base": "main", "body": preparation_body(plan, name)})
    print("Preparation PR: " + pr["html_url"])
    start_pr_checks(pr, api)
    print("Native PR checks activated. Review and merge the preparation PR.")


def prepare(args, api):
    modules = release.manifest()
    requested = scope_modules(modules, args.module, args.all)
    if args.issue is not None and args.issue < 1:
        raise ValueError("Use a positive release tracking issue number.")
    target = release.git("rev-parse", "HEAD")
    if target != release.git("rev-parse", "origin/main") or release.git("status", "--porcelain"):
        raise ValueError("Prepare from a clean checkout of current origin/main; no manual version edits are needed.")
    if args.local and not args.issue:
        raise ValueError("--local requires an existing --issue and creates no GitHub issue or PR.")
    name = args.plan or "auto-" + hashlib.sha256((",".join(sorted(requested)) + ":" + args.bump + ":" + str(args.auto_publish)).encode()).hexdigest()[:10] + "-" + target[:12]
    plan_path = release.plan_path(name)
    branch = "release/" + name
    if not args.local:
        release.authorize_actor(api)
        existing = list(api.pages("pulls?state=open&head=" + quote(release.REPOSITORY.split("/")[0] + ":" + branch, safe="")))
        if existing:
            print(f"Preparation already exists: {existing[0]['html_url']}")
            start_pr_checks(existing[0], api, resume=True)
            return
        remote_branch = api.repo("git/ref/heads/" + branch, missing=True)
        if remote_branch:
            release.git("-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "fetch", "origin", "refs/heads/" + branch)
            sha = remote_branch["object"]["sha"]
            existing_plan = json.loads(release.git("show", f"{sha}:releases/{name}.json"))
            if existing_plan["source_sha"] != target or set(existing_plan["requested_modules"]) != set(requested) or existing_plan.get("auto_publish", False) != args.auto_publish or release.git("rev-parse", sha + "^1") != target:
                raise ValueError("Existing preparation branch conflicts with this request; it was not overwritten.")
            issue = api.repo(f"issues/{existing_plan['issue']}")
            if issue["state"] != "open" or "pull_request" in issue:
                raise ValueError("Existing preparation branch needs an open tracking issue.")
            open_preparation(existing_plan, name, branch, api)
            return
    if plan_path.exists():
        raise ValueError("This plan already exists; select a new plan name or resume its preparation PR.")
    overrides = json.loads((release.ROOT / args.overrides).read_text(encoding="utf-8")) if args.overrides else {}
    releases = list(api.pages("releases"))
    published = {item["tag_name"] for item in releases if not item["draft"]}
    reserved_tags = {item["name"] for item in api.pages("tags")} | {item["tag_name"] for item in releases} | set(release.git("tag", "--list").splitlines())
    available_cache = {}
    def available(tag):
        if tag not in available_cache:
            available_cache[tag] = tag in published and bool(release.remote_tag(api, tag))
        return available_cache[tag]
    requirements = module_requirements(modules)
    directories = select_dependencies(requested, modules, requirements, available)
    history = History(api, modules, target, overrides)
    plan = {"schema_version": 1, "issue": args.issue or 1, "source_sha": target, "auto_publish": args.auto_publish,
            "requested_modules": requested, "included_dependencies": [directory for directory in directories if directory not in requested], "bump": args.bump, "modules": []}
    for directory in directories:
        previous = release.previous_release(directory, target, releases)
        entries, reviewed, direct, generated = history.notes(previous)
        related = [entry for entry in entries if entry["module"] == directory]
        prefix = "" if directory == "." else directory + "/"
        reserved = [tag[len(prefix):] for tag in reserved_tags if tag.startswith(prefix) and release.VERSION.fullmatch(tag[len(prefix):])]
        version = next_version(modules[directory]["version"], previous, args.bump, related, reserved)
        initial = overrides.get("initial_summaries", {}).get(directory)
        if previous is None and not initial:
            initial = f"Initial public source release of the {directory if directory != '.' else 'Commons'} module. Review supported behavior and compatibility limits in the module documentation; full cross-language parity is not implied."
        plan["modules"].append({"directory": directory, "version": version, "previous_tag": previous, "initial_summary": initial,
                                "entries": related, "reviewed_prs": reviewed, "excluded_prs": sorted(set(reviewed) - {entry["pr"] for entry in related}),
                                "untracked_commits": direct, "generated_history": generated, "dependency_updates": []})
    selected = {item["directory"]: item for item in plan["modules"]}
    predicted = deepcopy(requirements)
    version_paths = {"github.com/" + release.REPOSITORY + ("" if directory == "." else "/" + directory): item["version"] for directory, item in selected.items()}
    for values in predicted.values():
        for requirement in values:
            if requirement["Path"] in version_paths:
                requirement["Version"] = version_paths[requirement["Path"]]
    release.dependency_order(selected, modules, predicted, available)
    issue = api.repo(f"issues/{args.issue}") if args.issue else None
    if issue and ("pull_request" in issue or issue["state"] != "open"):
        raise ValueError("Use a real, open release tracking issue.")
    metadata = [release.ROOT / "tools/modules.json", release.ROOT / "go.work", release.ROOT / "go.work.sum"]
    metadata.extend(release.ROOT / directory / filename for directory in modules for filename in ("go.mod", "go.sum"))
    before = {path: path.read_bytes() if path.exists() else None for path in metadata}
    try:
        updates = synchronize_metadata(modules, selected, requirements)
        plan["metadata_updates"] = updates
        for item in plan["modules"]:
            item["dependency_updates"] = updates.get(item["directory"], [])
            item["notes"] = render_notes(release.REPOSITORY, item["directory"], item["version"], item["previous_tag"], item["entries"], item["initial_summary"], item["untracked_commits"], item["dependency_updates"])
        release.validate_plan(plan, release.manifest())
        if not issue:
            versions = "\n".join(f"- `{item['directory']}`: `{item['version']}`" for item in plan["modules"])
            body = f"## Modules and target versions\n\n{versions}\n\n## Release scope and compatibility\n\nRequested modules: {', '.join(requested)}. Unpublished dependencies are included automatically. Review initial capability statements and compatibility boundaries in the preparation PR.\n\n## Previous versions and accumulated changes\n\nSource: `{target}`. Each module uses its own previous published tag; the reviewed plan contains accumulated PR notes, generated historical summaries, and direct-commit acknowledgements.\n\n## Publication acceptance\n\nRequire contribution, module, and documentation checks; review the plan; merge the preparation PR; publish with GoReleaser and verify fresh public Go consumers. Keep this issue open until the entire batch succeeds. Automatic publication after merge: {'enabled' if args.auto_publish else 'disabled'}."
            issue = api.repo("issues", method="POST", data={"title": "[Release]: " + name, "body": body})
            plan["issue"] = issue["number"]
        release.write_json(plan_path, plan)
    except (RuntimeError, ValueError, KeyError, OSError, TypeError):
        for path, content in before.items():
            if content is None:
                path.unlink(missing_ok=True)
            else:
                path.write_bytes(content)
        plan_path.unlink(missing_ok=True)
        raise
    print("Prepared " + str(plan_path.relative_to(release.ROOT)))
    for item in plan["modules"]:
        print(f"  {item['directory']}: {item['previous_tag'] or 'unreleased'} -> {item['version']}")
    if args.local:
        print("Local metadata and plan are ready for review; no GitHub writes were made.")
        return
    changed = sorted(set(release.git("diff", "--name-only").splitlines()) | {path.relative_to(release.ROOT).as_posix() for path, content in before.items() if content is None and path.exists()})
    if not set(changed) <= {path.relative_to(release.ROOT).as_posix() for path in metadata}:
        raise ValueError("Unexpected changes outside release metadata; review the checkout before committing.")
    release.git("switch", "-c", branch)
    release.git("add", "--", *changed, plan_path.relative_to(release.ROOT).as_posix())
    release.git("-c", "user.name=github-actions[bot]", "-c", "user.email=41898282+github-actions[bot]@users.noreply.github.com", "commit", "-m", "chore(release): prepare " + name)
    release.git("-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "origin", branch)
    open_preparation(plan, name, branch, api)
