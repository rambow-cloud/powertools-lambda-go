"""Prepare, validate, and explicitly publish reviewed independent module releases."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import Request, urlopen

from release_notes import CATEGORIES, render_notes, render_unified_notes, validate_direct_note


ROOT = Path(__file__).resolve().parents[1]
REPOSITORY = "rambow-cloud/powertools-lambda-go"
GORELEASER_VERSION = "2.18.2"
MAIN_CHECKS = ("CI gate", "Full regression", "Modules and Lambda artifacts", "Runtime simulation", "DynamoDB Local",
               "Documentation / Build documentation", "Automation / Release tooling")
PR_CHECKS = ("PR contribution policy", *MAIN_CHECKS)
SHA = re.compile(r"[0-9a-f]{40}")
NAME = re.compile(r"[a-z0-9][a-z0-9-]{0,79}")
VERSION = re.compile(r"v(0|1)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?")


def run(*args, cwd=ROOT, env=None, input=None, include_stderr=False):
    result = subprocess.run(args, cwd=cwd, env=env, input=input, text=True, encoding="utf-8", capture_output=True)
    if result.returncode:
        raise RuntimeError(f"Command failed: {' '.join(args)}\n{result.stdout}\n{result.stderr}")
    return (result.stdout + result.stderr if include_stderr else result.stdout).strip()


def git(*args):
    return run("git", *args, cwd=ROOT)


class GitHub:
    def __init__(self):
        self.token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN") or run("gh", "auth", "token")

    def request(self, path, method="GET", data=None, missing=False):
        request = Request(
            "https://api.github.com/" + path,
            data=None if data is None else json.dumps(data).encode(),
            method=method,
            headers={"Authorization": "Bearer " + self.token, "Accept": "application/vnd.github+json", "Content-Type": "application/json", "X-GitHub-Api-Version": "2022-11-28"},
        )
        try:
            with urlopen(request, timeout=45) as response:
                body = response.read()
                return json.loads(body) if body else None
        except HTTPError as error:
            if missing and error.code == 404:
                return None
            raise RuntimeError(f"GitHub {method} {path} failed (HTTP {error.code}); resolve permissions or service availability before retrying.") from None
        except (URLError, TimeoutError):
            raise RuntimeError(f"GitHub {path} is unavailable; no automatic retries were attempted.") from None

    def repo(self, path, **kwargs):
        return self.request(f"repos/{REPOSITORY}/{path}", **kwargs)

    def pages(self, path):
        separator = "&" if "?" in path else "?"
        page = 1
        while True:
            items = self.repo(f"{path}{separator}per_page=100&page={page}")
            if not isinstance(items, list):
                raise RuntimeError(f"Expected a paginated list: {path}")
            yield from items
            if len(items) < 100:
                break
            page += 1


def manifest():
    data = json.loads((ROOT / "tools/modules.json").read_text(encoding="utf-8"))
    if data["base"] != "github.com/" + REPOSITORY:
        raise ValueError("Unexpected publication repository.")
    modules = {module["directory"]: module for module in data["modules"]}
    if "release_version" in data:
        version_key(data["release_version"])
        if any(module["version"] != data["release_version"] for module in modules.values() if module["public"] and module.get("status") != "deprecated-frozen"):
            raise ValueError("Maintained module versions must match the project release_version.")
    return modules


def manifest_version():
    return json.loads((ROOT / "tools/modules.json").read_text(encoding="utf-8")).get("release_version")


def tag_name(directory, version):
    return version if directory == "." else f"{directory}/{version}"


def version_key(version):
    match = VERSION.fullmatch(version)
    if not match:
        raise ValueError(f"Unsupported version: {version}; v2+ requires a separate module-path migration.")
    major, minor, patch, suffix = match.groups()
    identifiers = []
    for item in (suffix or "").split("."):
        if item.isdigit():
            if len(item) > 1 and item.startswith("0"):
                raise ValueError("Numeric prerelease identifiers cannot have leading zeros.")
            identifiers.append((0, int(item)))
        else:
            identifiers.append((1, item))
    return int(major), int(minor), int(patch), suffix is None, tuple(identifiers)


def commit_of_tag(tag):
    return git("rev-parse", "--verify", f"refs/tags/{tag}^{{commit}}")


def is_ancestor(base, target):
    return subprocess.run(["git", "merge-base", "--is-ancestor", base, target], cwd=ROOT, capture_output=True).returncode == 0


def previous_release(directory, target, releases, prerelease=False):
    prefix = "" if directory == "." else directory + "/"
    matches = []
    for release in releases:
        tag = release["tag_name"]
        if release["draft"] or (release["prerelease"] and not prerelease) or not tag.startswith(prefix):
            continue
        version = tag[len(prefix):]
        if not VERSION.fullmatch(version):
            continue
        sha = commit_of_tag(tag)
        if is_ancestor(sha, target):
            matches.append((version_key(version), tag))
    return max(matches)[1] if matches else None


def merged_prs(api, base, target, cache=None):
    """Use commit ancestry, not merge dates or the repository's latest Release."""
    args = ["rev-list", "--first-parent", "--reverse", target]
    if base:
        args.append("^" + base)
    commits = git(*args).splitlines()
    reachable = set(git("rev-list", target, *(["^" + base] if base else [])).splitlines())
    result, uncovered = {}, []
    cache = {} if cache is None else cache
    for sha in commits:
        if sha not in cache:
            cache[sha] = list(api.pages(f"commits/{sha}/pulls"))
        matches = [pr for pr in cache[sha] if pr.get("merged_at") and pr["base"]["ref"] == "main" and pr["base"]["repo"]["full_name"] == REPOSITORY and pr.get("merge_commit_sha") in reachable]
        if not matches:
            uncovered.append(sha)
        for pr in matches:
            result[pr["number"]] = pr
    return sorted(result.values(), key=lambda pr: (pr["merged_at"], pr["number"])), uncovered


def plan_path(name):
    if not NAME.fullmatch(name):
        raise ValueError("Plan name must be a lowercase slug (maximum 80 characters).")
    return ROOT / "releases" / (name + ".json")


def write_json(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    temporary.replace(path)


def validate_plan(plan, modules, project_version=None):
    if plan.get("preview"):
        raise ValueError("A notes preview is not a reviewed publication plan.")
    if plan.get("notes_format", 1) not in (1, 2):
        raise ValueError("Unsupported release-notes format.")
    if plan.get("schema_version") not in {1, 2} or not isinstance(plan.get("issue"), int) or plan["issue"] < 1 or not SHA.fullmatch(plan.get("source_sha", "")):
        raise ValueError("Invalid release plan identity.")
    unified = plan["schema_version"] == 2
    if project_version is not None and (not unified or plan.get("release_version") != project_version):
        raise ValueError("Current release policy requires a unified plan matching release_version.")
    if unified:
        version_key(plan.get("release_version", ""))
        cohort = {directory for directory, module in modules.items() if module["public"] and module.get("status") != "deprecated-frozen"}
        if "." not in cohort or {item["directory"] for item in plan.get("modules", [])} != cohort or any(item["version"] != plan["release_version"] for item in plan.get("modules", [])):
            raise ValueError("Unified plans must include every maintained public module at one version.")
        reviewed = next(item["reviewed_prs"] for item in plan["modules"] if item["directory"] == ".")
        for entry in plan.get("repository_entries", []):
            if entry["module"] != "repository" or entry["type"] not in CATEGORIES or entry["pr"] not in reviewed or not entry["description"].strip():
                raise ValueError("Invalid reviewed repository note.")
    if not isinstance(plan.get("auto_publish", False), bool):
        raise ValueError("Automatic publication authorization must be a boolean.")
    selected = {}
    for item in plan.get("modules", []):
        directory = item["directory"]
        module = modules.get(directory)
        if directory in selected or not module or not module["public"] or module.get("status") == "deprecated-frozen":
            raise ValueError(f"Not a unique maintained public module: {directory}")
        if item["version"] != module["version"]:
            raise ValueError(f"Release plan and module manifest disagree: {directory}")
        version_key(item["version"])
        previous = item["previous_tag"]
        if previous:
            old = previous.removeprefix("" if directory == "." else directory + "/")
            if tag_name(directory, old) != previous or version_key(item["version"]) <= version_key(old):
                raise ValueError(f"Invalid previous version for {directory}")
        for entry in item["entries"]:
            if entry["module"] != directory or entry["type"] not in CATEGORIES or not isinstance(entry["pr"], int) or entry["pr"] < 1 or not entry["description"].strip():
                raise ValueError(f"Invalid frozen note for {directory}")
        reviewed = item["reviewed_prs"]
        related = {entry["pr"] for entry in item["entries"]}
        if len(reviewed) != len(set(reviewed)) or any(not isinstance(number, int) or number < 1 for number in reviewed) or not related <= set(reviewed) or set(item["excluded_prs"]) != set(reviewed) - related:
            raise ValueError(f"Inconsistent reviewed/excluded PR coverage: {directory}")
        for sha, note in item["untracked_commits"].items():
            if not SHA.fullmatch(sha):
                raise ValueError("Invalid untracked commit SHA.")
            validate_direct_note(note, modules)
        updates = item.get("dependency_updates", [])
        for update in updates:
            if not isinstance(update, dict) or not update.get("path", "").startswith("github.com/" + REPOSITORY):
                raise ValueError("Invalid internal dependency update.")
            dependency = update["path"].removeprefix("github.com/" + REPOSITORY).removeprefix("/") or "."
            if dependency not in modules or update["to"] != modules[dependency]["version"]:
                raise ValueError("Dependency update does not match module metadata.")
            version_key(update["from"])
            version_key(update["to"])
        changed_files = None
        if plan.get("notes_format") == 2:
            from release_prepare import module_changes
            changed_files = item.get("changed_files")
            if changed_files != module_changes(modules, directory, previous, plan["source_sha"]):
                raise ValueError(f"Frozen module file changes do not match Git history: {directory}")
        expected = render_notes(REPOSITORY, directory, item["version"], previous, item["entries"], item["initial_summary"], item["untracked_commits"], updates, changed_files)
        if unified and directory == ".":
            expected = render_unified_notes(REPOSITORY, plan)
        if item["notes"] != expected:
            raise ValueError(f"Frozen notes do not match reviewed entries: {directory}")
        selected[directory] = item
    if not selected:
        raise ValueError("Empty release plan.")
    return selected


def dependency_order(selected, modules, requirements, available):
    paths = {"github.com/" + REPOSITORY + ("" if directory == "." else "/" + directory): directory for directory in modules}
    pending = {directory: set() for directory in selected}
    for directory in selected:
        for requirement in requirements[directory]:
            path, version = requirement["Path"], requirement["Version"]
            if path not in paths:
                if path == "github.com/" + REPOSITORY or path.startswith("github.com/" + REPOSITORY + "/"):
                    raise ValueError(f"Unknown internal dependency: {path}")
                continue
            dependency = paths[path]
            if not modules[dependency]["public"] or modules[dependency].get("status") == "deprecated-frozen":
                raise ValueError(f"Unsupported release dependency: {dependency}")
            tag = tag_name(dependency, version)
            if dependency in selected and selected[dependency]["version"] == version:
                pending[directory].add(dependency)
            elif not available(tag):
                raise ValueError(f"Include {dependency}@{version} explicitly in the plan or publish it first.")
    result = []
    while pending:
        ready = sorted(directory for directory, dependencies in pending.items() if not dependencies)
        if not ready:
            raise ValueError("Release dependency cycle.")
        for directory in ready:
            result.append(directory)
            del pending[directory]
        for dependencies in pending.values():
            dependencies.difference_update(ready)
    return result


def validate_unified_requirements(version, requirements):
    for values in requirements.values():
        for requirement in values:
            path = requirement["Path"]
            if (path == "github.com/" + REPOSITORY or path.startswith("github.com/" + REPOSITORY + "/")) and requirement["Version"] != version:
                raise ValueError("Unified internal requirements must use the cohort version.")


def latest_checks(api, sha):
    runs = []
    page = 1
    while True:
        response = api.repo(f"commits/{sha}/check-runs?per_page=100&page={page}")
        runs.extend(response["check_runs"])
        if len(response["check_runs"]) < 100:
            break
        page += 1
    latest = {}
    for check in runs:
        if check["app"]["slug"] == "github-actions" and check["id"] > latest.get(check["name"], {}).get("id", -1):
            latest[check["name"]] = check
    return latest


def check_runs(api, sha, names):
    checks = latest_checks(api, sha)
    for name in names:
        latest = checks.get(name)
        if not latest or latest["status"] != "completed" or latest["conclusion"] != "success":
            raise ValueError(f"Required check '{name}' has not passed for {sha}; wait for CI before dispatching.")


def remote_tag(api, tag):
    ref = api.repo("git/ref/tags/" + quote(tag, safe="/"), missing=True)
    if not ref:
        return None
    obj = ref["object"]
    while obj["type"] == "tag":
        obj = api.repo("git/tags/" + obj["sha"])["object"]
    if obj["type"] != "commit":
        raise ValueError(f"Tag does not identify a commit: {tag}")
    return obj["sha"]


def release_for_tag(api, tag):
    """GitHub's tag endpoint returns published Releases, not drafts."""
    published = api.repo("releases/tags/" + quote(tag, safe=""), missing=True)
    if published:
        return published
    drafts = [item for item in api.pages("releases") if item["tag_name"] == tag]
    if len(drafts) > 1:
        raise ValueError(f"Multiple Releases match {tag}; resolve the conflict before publishing.")
    return drafts[0] if drafts else None


def await_release_phase(api, directory, item, sha, draft, wait_seconds=30):
    """Wait for a successful GoReleaser write to become visible to API reads."""
    tag = tag_name(directory, item["version"])
    deadline = time.monotonic() + wait_seconds
    while True:
        observed = release_for_tag(api, tag)
        check_existing(directory, item, sha, sha, observed)
        if observed and observed["draft"] == draft:
            return observed
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            phase = "draft" if draft else "published Release"
            raise ValueError(f"GoReleaser's expected {phase} is not visible after {wait_seconds}s: {tag}; preserve existing objects and resume after investigating.")
        time.sleep(min(2, remaining))


def check_existing(directory, item, sha, existing, release):
    tag = tag_name(directory, item["version"])
    if existing and existing != sha:
        raise ValueError(f"Existing tag {tag} points to a different commit; never rewrite it.")
    if release and (not existing or (release["body"] or "").rstrip("\r\n") != item["notes"].rstrip("\r\n") or release["name"] != tag or release["prerelease"] != ("-" in item["version"])):
        raise ValueError(f"Existing Release conflicts with the reviewed plan: {tag}")


def authorize_actor(api):
    actor = os.environ.get("GITHUB_ACTOR") or api.request("user")["login"]
    permission = api.repo(f"collaborators/{quote(actor, safe='')}/permission")["permission"]
    if permission not in {"admin", "maintain", "write"}:
        raise ValueError("Release automation requires repository write permission.")
    return actor


def preflight(args, api):
    if not SHA.fullmatch(args.sha):
        raise ValueError("Use the complete 40-character merged commit SHA.")
    if git("rev-parse", "HEAD") != args.sha:
        raise ValueError("Checkout the exact merged preparation SHA before preflight.")
    if git("status", "--porcelain"):
        raise ValueError("Release checkout must be clean; commit reviewed changes before preflight.")
    plan = json.loads(plan_path(args.plan).read_text(encoding="utf-8"))
    modules = manifest()
    selected = validate_plan(plan, modules, manifest_version())
    if plan["issue"] != args.issue:
        raise ValueError("Dispatch issue does not match the reviewed plan.")
    authorize_actor(api)
    pr = api.repo(f"pulls/{args.pr}")
    if not pr["merged"] or pr["base"]["ref"] != "main" or pr["base"]["repo"]["full_name"] != REPOSITORY or pr["merge_commit_sha"] != args.sha:
        raise ValueError("Expected a preparation PR merged into this repository's main at the supplied SHA.")
    body = re.sub(r"<!--.*?(?:-->|\Z)", "", pr.get("body") or "", flags=re.S)
    if not re.search(rf"(?im)^Refs #{args.issue}[ \t]*$", body) or re.search(rf"(?im)^(?:close[sd]?|fix(?:es|ed)?|resolve[sd]?) #{args.issue}[ \t]*$", body):
        raise ValueError("Preparation PR must use 'Refs #ISSUE' and leave the release issue open.")
    issue = api.repo(f"issues/{args.issue}")
    if "pull_request" in issue:
        raise ValueError("Publication tracking reference is a PR, not an issue.")
    if not is_ancestor(args.sha, "origin/main"):
        raise ValueError("Release commit is not on main.")
    if git("rev-parse", args.sha + "^1") != plan["source_sha"]:
        raise ValueError("Preparation history changed since notes were generated. Refresh the plan and squash/merge the preparation PR against that source SHA.")
    allowed = {f"releases/{args.plan}.json", "tools/modules.json", "go.work", "go.work.sum"}
    allowed.update(f"{directory + '/' if directory != '.' else ''}{filename}" for directory in modules for filename in ("go.mod", "go.sum", "LICENSE", "NOTICE"))
    changed = set(git("diff", "--name-only", plan["source_sha"], args.sha).splitlines())
    if not changed <= allowed or f"releases/{args.plan}.json" not in changed:
        raise ValueError("Preparation PR must only change its release plan and module/dependency/license metadata; ship code changes in earlier PRs.")
    check_runs(api, pr["head"]["sha"], PR_CHECKS)
    check_runs(api, args.sha, MAIN_CHECKS)
    requirements = {}
    env = os.environ.copy()
    env.update(CGO_ENABLED="0", GOWORK="off")
    for directory in selected:
        data = json.loads(run("go", "mod", "edit", "-json", cwd=ROOT / directory, env=env))
        expected = "github.com/" + REPOSITORY + ("" if directory == "." else "/" + directory)
        if data["Module"]["Path"] != expected or data.get("Replace"):
            raise ValueError(f"Invalid module path or replace directives: {directory}")
        requirements[directory] = data.get("Require") or []
        for update in selected[directory].get("dependency_updates", []):
            if not any(requirement["Path"] == update["path"] and requirement["Version"] == update["to"] for requirement in requirements[directory]):
                raise ValueError("Reviewed dependency updates disagree with go.mod.")
    if plan["schema_version"] == 2:
        validate_unified_requirements(plan["release_version"], requirements)
    def available(tag):
        release = release_for_tag(api, tag)
        return bool(release and not release["draft"] and remote_tag(api, tag))
    order = dependency_order(selected, modules, requirements, available)
    releases = list(api.pages("releases"))
    for directory in order:
        item = selected[directory]
        # Ignore this exact version when resuming a partially completed batch.
        prior = [release for release in releases if release["tag_name"] != tag_name(directory, item["version"])]
        if previous_release(directory, plan["source_sha"], prior, prerelease="-" in item["version"]) != item["previous_tag"]:
            raise ValueError(f"Previous published release changed: {directory}; prepare a fresh plan.")
        tag = tag_name(directory, item["version"])
        if git("tag", "--list", tag) and commit_of_tag(tag) != args.sha:
            raise ValueError(f"Local tag {tag} conflicts with the reviewed SHA.")
        existing = remote_tag(api, tag)
        release = release_for_tag(api, tag)
        check_existing(directory, item, args.sha, existing, release)
        if issue["state"] != "open" and not (existing and release and not release["draft"]):
            raise ValueError("Release tracking issue is closed before publication completed.")
    return plan, selected, modules, order


def verify_consumer(directory, item, module, session):
    name = "github.com/" + REPOSITORY + ("" if directory == "." else "/" + directory)
    consumer = session / "consumers" / ("commons" if directory == "." else directory)
    consumer.mkdir(parents=True, exist_ok=True)
    consumer.joinpath("go.mod").write_text(f"module example.com/release-consumer\n\ngo 1.27\n\nrequire {name} {item['version']}\n", encoding="utf-8")
    imported = name + ("/" + module["package"] if module.get("package") else "")
    consumer.joinpath("main.go").write_text(f'package main\n\nimport _ "{imported}"\n\nfunc main() {{}}\n', encoding="utf-8")
    env = os.environ.copy()
    for variable in ("GOFLAGS", "GOOS", "GOARCH", "GOPRIVATE", "GONOPROXY", "GONOSUMDB", "GOTOOLCHAIN", "GOENV"):
        env.pop(variable, None)
    env.update(CGO_ENABLED="0", GOWORK="off", GOENV="off", GOTOOLCHAIN="local", GOPROXY="https://proxy.golang.org", GOSUMDB="sum.golang.org", GOMODCACHE=str(session / "cache"), GOCACHE=str(session / "build-cache"))
    if module.get("test_os") == "linux":
        env.update(GOOS="linux", GOARCH="amd64")
    downloaded = json.loads(run("go", "mod", "download", "-json", name, cwd=consumer, env=env))
    run("go", "mod", "tidy", cwd=consumer, env=env)
    run("go", "build", "-mod=readonly", "-o", str(consumer / "consumer"), ".", cwd=consumer, env=env)
    run("go", "mod", "verify", cwd=consumer, env=env)
    graph = json.JSONDecoder()
    text = run("go", "list", "-m", "-json", "all", cwd=consumer, env=env)
    while text.strip():
        record, end = graph.raw_decode(text.lstrip())
        if record.get("Replace"):
            raise ValueError("Published consumer unexpectedly used replacement modules.")
        text = text.lstrip()[end:]
    return {"module": directory, "version": item["version"], "sum": downloaded["Sum"], "go_mod_sum": downloaded["GoModSum"], "consumer_build": True, "checksum_verification": True}


def goreleaser_config(directory, item, output):
    """Derive project/output paths and explicit prerelease status from the reviewed plan."""
    name = "commons" if directory == "." else directory.replace("/", "-")
    config = json.loads((ROOT / ".goreleaser.json").read_text(encoding="utf-8"))
    config.update(project_name=name, dist=str(output / name / "artifacts"))
    config["release"]["prerelease"] = str("-" in item["version"]).lower()
    path = output / name / "goreleaser.json"
    write_json(path, config)
    return path


def run_goreleaser(directory, item, args, output, config, token=None, draft=True):
    """The reviewed preflight replaces OSS's incompatible prefixed-tag validation."""
    env = os.environ.copy()
    for variable in ("GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN", "GITEA_TOKEN", "GORELEASER_KEY"):
        env.pop(variable, None)
    env.update(CGO_ENABLED="0", GORELEASER_CURRENT_TAG=tag_name(directory, item["version"]),
               GORELEASER_PREVIOUS_TAG=item["previous_tag"] or args.sha)
    if token:
        env["GITHUB_TOKEN"] = token
    name = config.parent.name
    stage = "preflight" if not token else "draft" if draft else "publish"
    # --draft=false does not override draft:true in this pinned OSS version.
    phase_config = config.parent / ("goreleaser-" + stage + ".json")
    configuration = json.loads(config.read_text(encoding="utf-8"))
    configuration["release"]["draft"] = draft
    configuration["release"]["make_latest"] = bool(getattr(args, "unified", False) and directory == "." and not draft and "-" not in item["version"])
    write_json(phase_config, configuration)
    command = ["goreleaser", "release", "--clean", "--config", str(phase_config),
               "--release-notes", str(output / (name + ".md")),
               "--skip=validate,announce" if token else "--skip=validate,publish,announce",
               "--fail-fast"]
    log = config.parent / (stage + ".log")
    try:
        result = run(*command, cwd=ROOT, env=env, include_stderr=True)
    except RuntimeError as error:
        log.write_text(str(error) + "\n", encoding="utf-8")
        raise
    log.write_text(result + "\n", encoding="utf-8")
    print(f"GoReleaser {stage}: {tag_name(directory, item['version'])}; output saved to {log}", flush=True)


def publish(args, api):
    plan, selected, modules, order = preflight(args, api)
    args.unified = plan["schema_version"] == 2
    output = ROOT / "dist/releases" / args.plan
    output.mkdir(parents=True, exist_ok=True)
    digest = hashlib.sha256(plan_path(args.plan).read_bytes()).hexdigest()
    report = {"plan": args.plan, "plan_sha256": digest, "source_sha": args.sha, "issue": args.issue, "preparation_pr": args.pr, "publish": args.publish, "completed": False, "modules": []}
    write_json(output / "progress.json", report)
    version = run("goreleaser", "--version")
    if not re.search(rf"(?m)^GitVersion:\s+v?{re.escape(GORELEASER_VERSION)}\s*$", version):
        raise ValueError(f"Install GoReleaser OSS v{GORELEASER_VERSION}; the release workflow pins this version.")
    configs = {}
    for directory in order:
        item = selected[directory]
        print(f"{'Publish' if args.publish else 'Preflight'}: {tag_name(directory, item['version'])}", flush=True)
        note_path = output / (("commons" if directory == "." else directory.replace("/", "-")) + ".md")
        note_path.write_text(item["notes"], encoding="utf-8", newline="\n")
        configs[directory] = goreleaser_config(directory, item, output)
        run("goreleaser", "check", "--config", str(configs[directory]))
        run_goreleaser(directory, item, args, output, configs[directory])
    if not args.publish:
        report.update(completed=True, publication="No tags, Releases, or issue changes were made.")
        write_json(output / "progress.json", report)
        print(report["publication"])
        return
    session = Path(tempfile.mkdtemp(prefix="consumer-", dir=output))
    deferred_root = None
    try:
        for directory in order:
            item = selected[directory]
            tag = tag_name(directory, item["version"])
            record = {"tag": tag, "tag_created": False, "consumer_verified": False, "release_published": False}
            report["modules"].append(record)
            existing = remote_tag(api, tag)
            check_existing(directory, item, args.sha, existing, None)
            if git("tag", "--list", tag):
                if commit_of_tag(tag) != args.sha:
                    raise ValueError(f"Local tag {tag} conflicts with the reviewed SHA.")
            else:
                git("tag", tag, args.sha)
            if not existing:
                api.repo("git/refs", method="POST", data={"ref": "refs/tags/" + tag, "sha": args.sha})
            record["tag_created"] = True
            write_json(output / "progress.json", report)
            release = release_for_tag(api, tag)
            check_existing(directory, item, args.sha, args.sha, release)
            if not release:
                run_goreleaser(directory, item, args, output, configs[directory], token=api.token)
                release = await_release_phase(api, directory, item, args.sha, draft=True)
            evidence = verify_consumer(directory, item, modules[directory], session)
            record.update(consumer_verified=True, evidence=evidence)
            write_json(output / "progress.json", report)
            if release["draft"]:
                if plan["schema_version"] == 2 and directory == ".":
                    deferred_root = record
                else:
                    run_goreleaser(directory, item, args, output, configs[directory], token=api.token, draft=False)
                    release = await_release_phase(api, directory, item, args.sha, draft=False)
            record.update(release_published=not release["draft"], url=release["html_url"])
            write_json(output / "progress.json", report)
        if deferred_root:
            run_goreleaser(".", selected["."], args, output, configs["."], token=api.token, draft=False)
            release = await_release_phase(api, ".", selected["."], args.sha, draft=False)
            deferred_root.update(release_published=True, url=release["html_url"])
            write_json(output / "progress.json", report)
        report["completed"] = True
        write_json(output / "progress.json", report)
        marker = f"<!-- release-plan:{args.plan}:{digest} -->"
        if not any(marker in (comment.get("body") or "") for comment in api.pages(f"issues/{args.issue}/comments")):
            links = "\n".join(f"- [{record['tag']}]({record['url']})" for record in report["modules"])
            api.repo(f"issues/{args.issue}/comments", method="POST", data={"body": f"{marker}\nPublished the reviewed plan at `{args.sha}`. All selected public consumers passed with CGO disabled, GOWORK=off, fresh caches, and checksum verification.\n\n{links}"})
        api.repo(f"issues/{args.issue}", method="PATCH", data={"state": "closed", "state_reason": "completed"})
    except (RuntimeError, ValueError, KeyError, OSError, TypeError) as error:
        report.update(completed=False, error=str(error))
        write_json(output / "progress.json", report)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    prepare_parser = commands.add_parser("prepare", help="Write a reviewed plan without publishing")
    prepare_parser.add_argument("--issue", type=int, help="Reuse an open release tracking issue; otherwise create one")
    prepare_parser.add_argument("--all", action="store_true", help="All maintained public modules (always selected)")
    prepare_parser.add_argument("--plan", help="Optional plan name; generated by default")
    prepare_parser.add_argument("--bump", choices=("auto", "patch", "minor", "major"), default="auto")
    prepare_parser.add_argument("--auto-publish", action="store_true", help="Authorize publication after this preparation PR merges and main CI passes")
    prepare_parser.add_argument("--local", action="store_true", help="Only write metadata/plan; requires an existing issue")
    prepare_parser.add_argument("--overrides", help="Reviewed historical summaries and initial release scope (JSON)")
    preview_parser = commands.add_parser("preview", help="Preview notes without changing module metadata or publishing")
    preview_parser.add_argument("--base", required=True, help="Published project version to compare against")
    preview_parser.add_argument("--target", default="origin/main", help="Committed source revision, frozen to its SHA")
    preview_parser.add_argument("--bump", choices=("auto", "patch", "minor", "major"), default="auto")
    preview_parser.add_argument("--draft", action="store_true", help="Create/update a separate GitHub notes-preview draft")
    publish_parser = commands.add_parser("publish", help="Preflight by default; --publish explicitly creates tags/Releases")
    publish_parser.add_argument("--issue", type=int)
    publish_parser.add_argument("--pr", type=int, required=True)
    publish_parser.add_argument("--sha")
    publish_parser.add_argument("--plan")
    publish_parser.add_argument("--publish", action="store_true")
    args = parser.parse_args()
    try:
        api = GitHub()
        if args.command == "prepare":
            from release_prepare import prepare
            prepare(args, api)
        elif args.command == "preview":
            from release_preview import preview
            preview(args, api)
        else:
            if not all((args.issue, args.sha, args.plan)):
                from release_automation import resolve_preparation, checkout_preparation
                resolved, _ = resolve_preparation(args.pr, api)
                for field in ("issue", "sha", "plan"):
                    supplied = getattr(args, field)
                    if supplied is not None and supplied != getattr(resolved, field):
                        raise ValueError("Publication input conflicts with the merged preparation PR.")
                    setattr(args, field, getattr(resolved, field))
                checkout_preparation(args.sha)
            publish(args, api)
    except (RuntimeError, ValueError, KeyError, OSError, TypeError) as error:
        raise SystemExit(str(error)) from None


if __name__ == "__main__":
    main()
