"""Resolve reviewed preparation PRs and publish authorized batches after main CI."""

import json
import os
from pathlib import Path
from types import SimpleNamespace

import release


def resolve_preparation(number, api):
    if not isinstance(number, int) or number < 1:
        raise ValueError("Use a positive preparation PR number.")
    pr = api.repo(f"pulls/{number}")
    if not pr.get("merged") or pr["base"]["ref"] != "main" or pr["base"]["repo"]["full_name"] != release.REPOSITORY:
        raise ValueError("Use a preparation PR merged into this repository's main.")
    sha = pr["merge_commit_sha"]
    if not release.SHA.fullmatch(sha) or not release.is_ancestor(sha, "origin/main"):
        raise ValueError("Preparation commit is not a full SHA on main.")
    files = list(api.pages(f"pulls/{number}/files"))
    plans = [file["filename"] for file in files if file.get("status") != "removed" and file["filename"].startswith("releases/") and file["filename"].endswith(".json")]
    if len(plans) != 1:
        raise ValueError("Preparation PR must contain exactly one release plan.")
    name = plans[0].removeprefix("releases/").removesuffix(".json")
    release.plan_path(name)
    plan = json.loads(release.git("show", f"{sha}:{plans[0]}"))
    if plan.get("schema_version") not in {1, 2} or not isinstance(plan.get("issue"), int) or plan["issue"] < 1:
        raise ValueError("Invalid preparation plan identity.")
    if not isinstance(plan.get("auto_publish", False), bool):
        raise ValueError("Invalid automatic publication authorization.")
    return SimpleNamespace(issue=plan["issue"], pr=number, sha=sha, plan=name, publish=False), plan


def checkout_preparation(sha):
    if release.git("status", "--porcelain"):
        raise ValueError("Publication requires a clean checkout.")
    release.git("checkout", "--detach", sha)


def automatic_preparation(event, api):
    workflow = event.get("workflow_run", {})
    if workflow.get("event") != "push" or workflow.get("head_branch") != "main" or workflow.get("head_repository", {}).get("full_name") != release.REPOSITORY:
        print("No publication: only same-repository main push checks are eligible.")
        return None
    sha = workflow.get("head_sha", "")
    if not release.SHA.fullmatch(sha) or not release.is_ancestor(sha, "origin/main"):
        raise ValueError("Workflow completion SHA is not on main.")
    candidates = [pr for pr in api.pages(f"commits/{sha}/pulls") if pr.get("merged_at") and pr.get("merge_commit_sha") == sha and pr["base"]["ref"] == "main" and pr["base"]["repo"]["full_name"] == release.REPOSITORY]
    if len(candidates) != 1:
        print("No publication: this commit is not one merged preparation PR.")
        return None
    number = candidates[0]["number"]
    files = list(api.pages(f"pulls/{number}/files"))
    if not any(file["filename"].startswith("releases/") and file["filename"].endswith(".json") for file in files):
        print("No publication: this PR has no release plan.")
        return None
    args, plan = resolve_preparation(number, api)
    if plan.get("auto_publish") is not True:
        print("No publication: automatic publication was not authorized in this plan.")
        return None
    if api.repo(f"issues/{args.issue}")["state"] != "open":
        print("No publication: the release tracking issue is already closed.")
        return None
    try:
        release.check_runs(api, sha, release.MAIN_CHECKS)
    except ValueError:
        print("No publication yet: required main checks have not all passed. Their next completion event will re-evaluate this batch.")
        return None
    args.publish = True
    return args


def main():
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text(encoding="utf-8"))
    api = release.GitHub()
    if os.environ["GITHUB_EVENT_NAME"] == "workflow_run":
        args = automatic_preparation(event, api)
        if args is None:
            return
    else:
        number = os.environ.get("RELEASE_PR", "")
        if not number.isdigit() or int(number) < 1:
            raise ValueError("Use a positive preparation PR number.")
        args, _ = resolve_preparation(int(number), api)
        args.publish = os.environ.get("RELEASE_PUBLISH") == "true"
    checkout_preparation(args.sha)
    release.publish(args, api)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, ValueError, KeyError, OSError, TypeError) as error:
        raise SystemExit(str(error)) from None
