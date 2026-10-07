"""Parse human-written PR notes and render deterministic module changelogs."""

import re


CATEGORIES = {
    "breaking": "Breaking changes",
    "feature": "Features",
    "fix": "Fixes",
    "documentation": "Documentation",
    "maintenance": "Maintenance",
}


def section(body, heading):
    text = re.sub(r"<!--.*?(?:-->|\Z)", "", body or "", flags=re.S)
    match = re.search(rf"(?im)^## {re.escape(heading)}[ \t]*\n", text)
    if not match:
        return ""
    return re.split(r"(?m)^## ", text[match.end():], maxsplit=1)[0].strip()


def parse_notes(body, directories):
    """Accept explicit entries or an explained absence; never infer a summary."""
    text = section(body, "Release notes")
    if re.fullmatch(r"None: [^\n]*[A-Za-z0-9][^\n]*", text):
        return []
    if not text:
        raise ValueError("Add '## Release notes' with entries or 'None: <reason>'.")
    entries = []
    for line in text.splitlines():
        if not line.strip():
            continue
        match = re.fullmatch(r"- ([^|\s]+) \| ([a-z]+) \| (\S.*)", line)
        if not match:
            raise ValueError("Release notes must use '- MODULE | TYPE | User-visible description.'")
        directory, kind, description = match.groups()
        if directory not in {*directories, "repository"}:
            raise ValueError(f"Unknown release-note module: {directory}")
        if kind not in CATEGORIES:
            raise ValueError(f"Unknown release-note type: {kind}")
        entries.append({"module": directory, "type": kind, "description": description})
    if not entries:
        raise ValueError("Release notes contain no entries.")
    return entries


def validate_direct_note(value, directories):
    if not isinstance(value, dict) or not isinstance(value.get("modules"), list) or not value["modules"] or not isinstance(value.get("description"), str) or not value["description"].strip():
        raise ValueError("Direct-commit notes need modules and an explained description.")
    if len(value["modules"]) != len(set(value["modules"])) or any(module not in {*directories, "repository"} for module in value["modules"]):
        raise ValueError("Direct-commit notes contain unknown or repeated modules.")


def render_notes(repository, module, version, previous_tag, entries, initial_summary=None, untracked_commits=None, dependency_updates=None, changed_files=None):
    if changed_files is not None:
        item = dict(directory=module, version=version, previous_tag=previous_tag, entries=entries,
                    initial_summary=initial_summary, untracked_commits=untracked_commits or {},
                    dependency_updates=dependency_updates or [], changed_files=changed_files)
        return render_component_notes(repository, item)
    tag = version if module == "." else f"{module}/{version}"
    lines = [f"# {tag}", ""]
    if previous_tag is None:
        if not initial_summary or not initial_summary.strip():
            raise ValueError(f"First release of {module} needs an initial scope and compatibility summary.")
        lines += ["## Initial release", "", initial_summary.strip(), ""]
    relevant = [entry for entry in entries if entry["module"] == module]
    for kind, title in CATEGORIES.items():
        selected = [entry for entry in relevant if entry["type"] == kind]
        if not selected:
            continue
        lines += [f"## {title}", ""]
        for entry in selected:
            reference = f"[#{entry['pr']}](https://github.com/{repository}/pull/{entry['pr']})"
            lines.append(f"- {entry['description']} ({reference})")
        lines.append("")
    direct = {sha: note for sha, note in (untracked_commits or {}).items() if module in note["modules"]}
    if direct:
        lines += ["## Changes without a pull request", ""]
        for sha, note in sorted(direct.items()):
            lines.append(f"- {note['description']} ([{sha[:7]}](https://github.com/{repository}/commit/{sha}))")
        lines.append("")
    if dependency_updates:
        lines += ["## Internal dependency updates", ""]
        for update in dependency_updates:
            lines.append(f"- `{update['path']}`: `{update['from']}` → `{update['to']}`.")
        lines.append("")
    if previous_tag:
        lines += [f"**Full changelog:** https://github.com/{repository}/compare/{previous_tag}...{tag}", ""]
    else:
        lines += [f"**Source:** https://github.com/{repository}/tree/{tag}", ""]
    lines += ["## Installation", "", "```sh", f"go get github.com/{repository}{'' if module == '.' else '/' + module}@{version}", "```", ""]
    return "\n".join(lines)


def render_unified_notes(repository, plan):
    """Render the root version's complete, component-grouped release summary."""
    if plan.get("notes_format") == 2:
        return render_file_based_notes(repository, plan)
    version = plan["release_version"]
    lines = [f"# {version}", "", "All maintained public modules use this version.", "",
             f"**Publication status:** https://github.com/{repository}/issues/{plan['issue']}", ""]
    if plan.get("publication_mode") == "project":
        lines += ["One project Release contains these notes; independent Go modules use matching tags at the same commit.", ""]
    for item in plan["modules"]:
        directory = item["directory"]
        direct = {sha: note for sha, note in item["untracked_commits"].items() if directory in note["modules"]}
        if not (item["entries"] or item["initial_summary"] or direct):
            continue
        lines += ["## " + ("Commons" if directory == "." else directory), ""]
        if item["initial_summary"]:
            lines += [item["initial_summary"].strip(), ""]
        for kind, title in CATEGORIES.items():
            entries = [entry for entry in item["entries"] if entry["type"] == kind]
            if entries:
                lines += [f"### {title}", ""]
                lines += [f"- {entry['description']} ([#{entry['pr']}](https://github.com/{repository}/pull/{entry['pr']}))" for entry in entries]
                lines.append("")
        if direct:
            lines += ["### Changes without a pull request", ""]
            lines += [f"- {note['description']} ([{sha[:7]}](https://github.com/{repository}/commit/{sha}))" for sha, note in sorted(direct.items())]
            lines.append("")
    root = next(item for item in plan["modules"] if item["directory"] == ".")
    repository_direct = {sha: note for sha, note in root["untracked_commits"].items() if "repository" in note["modules"]}
    if plan.get("repository_entries") or repository_direct:
        lines += ["## Repository", ""]
        for kind, title in CATEGORIES.items():
            entries = [entry for entry in plan.get("repository_entries", []) if entry["type"] == kind]
            if entries:
                lines += [f"### {title}", ""]
                lines += [f"- {entry['description']} ([#{entry['pr']}](https://github.com/{repository}/pull/{entry['pr']}))" for entry in entries]
                lines.append("")
        lines += [f"- {note['description']} ([{sha[:7]}](https://github.com/{repository}/commit/{sha}))" for sha, note in sorted(repository_direct.items())]
        lines.append("")
    lines += ["## Module versions", "", "| Module | Version | Changes |", "|---|---|---|"]
    for item in plan["modules"]:
        directory = item["directory"]
        tag = version if directory == "." else directory + "/" + version
        changed = item["entries"] or item["initial_summary"] or any(directory in note["modules"] for note in item["untracked_commits"].values())
        status = "Component changes" if changed else "Dependency alignment" if item.get("dependency_updates") else "No component changes"
        route = "tree" if plan.get("publication_mode") == "project" and directory != "." else "releases/tag"
        lines.append(f"| `{directory}` | [{version}](https://github.com/{repository}/{route}/{tag}) | {status} |")
    footer = "Internal dependency requirements are synchronized to this version."
    if plan.get("publication_mode") != "project":
        footer += " See each component Release for dependency details."
    lines += ["", footer, ""]
    if root["previous_tag"]:
        lines += [f"**Full changelog:** https://github.com/{repository}/compare/{root['previous_tag']}...{version}", ""]
    return "\n".join(lines)


def change_status(item):
    if item["previous_tag"] is None:
        return "Initial release"
    return "Updated" if item["changed_files"] else f"Bumped to {item['version']} only"


def details(repository, item, level, include_files=True):
    """Render real changes only; a broad PR note cannot turn an empty diff into a feature."""
    if include_files and item["previous_tag"] and not item["changed_files"]:
        return [f"Bumped to `{item['version']}` only. No module file changes.", ""]
    lines = []
    if item["previous_tag"] is None:
        if not item["initial_summary"]:
            raise ValueError("First release needs an initial scope and compatibility summary.")
        lines += [f"{level} Initial release", "", item["initial_summary"].strip(), ""]
    for kind, title in CATEGORIES.items():
        entries = [entry for entry in item["entries"] if entry["module"] == item["directory"] and entry["type"] == kind]
        if entries:
            lines += [f"{level} {title}", ""]
            lines += [f"- {entry['description']} ([#{entry['pr']}](https://github.com/{repository}/pull/{entry['pr']}))" for entry in entries]
            lines.append("")
    direct = {sha: note for sha, note in item["untracked_commits"].items() if item["directory"] in note["modules"]}
    if direct:
        lines += [f"{level} Changes without a pull request", ""]
        lines += [f"- {note['description']} ([{sha[:7]}](https://github.com/{repository}/commit/{sha}))" for sha, note in sorted(direct.items())]
        lines.append("")
    if include_files and item["changed_files"]:
        lines += [f"<details><summary>Changed files ({len(item['changed_files'])})</summary>", ""]
        lines += [f"- `{path}`" for path in item["changed_files"]]
        lines += ["", "</details>", ""]
    return lines


def render_component_notes(repository, item):
    directory, version = item["directory"], item["version"]
    tag = version if directory == "." else directory + "/" + version
    lines = [f"# {tag}", "", *details(repository, item, "##")]
    if item["previous_tag"]:
        lines += [f"**Full changelog:** https://github.com/{repository}/compare/{item['previous_tag']}...{tag}", ""]
    else:
        lines += [f"**Source:** https://github.com/{repository}/tree/{tag}", ""]
    lines += ["## Installation", "", "```sh", f"go get github.com/{repository}{'' if directory == '.' else '/' + directory}@{version}", "```", ""]
    return "\n".join(lines)


def module_display_name(repository, directory):
    project = repository.rsplit("/", 1)[-1]
    return project if directory == "." else f"{project}/{directory}"


def render_file_based_notes(repository, plan):
    version = plan["release_version"]
    preview = plan.get("preview", False)
    target = plan["source_sha"] if preview else version
    changed = [item for item in plan["modules"] if item["previous_tag"] is None or item["changed_files"]]
    unchanged = [item for item in plan["modules"] if item["previous_tag"] and not item["changed_files"]]
    lines = [f"# {version}" + (" — Release notes preview" if preview else ""), ""]
    if preview:
        lines += ["This is a documentation preview. Module versions and dependency metadata have not been prepared or published.",
                  "Use the reviewed release preparation workflow to publish after checking these notes.", ""]
    lines += ["All maintained public modules move to this version. Modules without file changes receive a version bump only.", "",
              f"**Scope:** {len(changed)} modules with changes; {len(unchanged)} version bumps only.", "",
              f"**Source:** [{plan['source_sha'][:12]}](https://github.com/{repository}/tree/{plan['source_sha']})", ""]
    if not preview:
        lines += [f"**Publication status:** https://github.com/{repository}/issues/{plan['issue']}", ""]
        if plan.get("publication_mode") == "project":
            lines += ["One project Release contains these notes; independent Go modules use matching tags at the same commit.", ""]
    if changed:
        lines += ["## Updated modules", ""]
        for item in changed:
            lines += ["### " + module_display_name(repository, item["directory"]), ""]
            lines += details(repository, item, "####")
    if unchanged:
        lines += ["## Version bumps only", "", "No module file changes; internal requirements will follow the unified version.", ""]
        lines += [f"- `{module_display_name(repository, item['directory'])}`: bumped to `{version}`." for item in unchanged]
        lines.append("")
    root = next(item for item in plan["modules"] if item["directory"] == ".")
    repository_item = {**root, "directory": "repository", "previous_tag": root["previous_tag"] or "initial",
                       "entries": plan.get("repository_entries", []), "changed_files": [], "initial_summary": None}
    repository_details = details(repository, repository_item, "###", include_files=False)
    if repository_details:
        lines += ["## Repository", "", *repository_details]
    lines += ["## Module versions", "", f"<details><summary>All maintained modules ({len(plan['modules'])})</summary>", "",
              "| Module | Version | Changes |", "|---|---|---|"]
    for item in plan["modules"]:
        directory = item["directory"]
        tag = version if directory == "." else directory + "/" + version
        route = "tree" if plan.get("publication_mode") == "project" and directory != "." else "releases/tag"
        label = f"`{version}`" if preview else f"[{version}](https://github.com/{repository}/{route}/{tag})"
        lines.append(f"| `{module_display_name(repository, directory)}` | {label} | {change_status(item)} |")
    lines += ["", "</details>"]
    if root["previous_tag"]:
        lines += ["", f"**Full changelog:** https://github.com/{repository}/compare/{root['previous_tag']}...{target}"]
    return "\n".join([*lines, ""])
