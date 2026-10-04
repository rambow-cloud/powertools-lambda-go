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


def render_notes(repository, module, version, previous_tag, entries, initial_summary=None, untracked_commits=None):
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
    if previous_tag:
        lines += [f"**Full changelog:** https://github.com/{repository}/compare/{previous_tag}...{tag}", ""]
    else:
        lines += [f"**Source:** https://github.com/{repository}/tree/{tag}", ""]
    lines += ["## Installation", "", "```sh", f"go get github.com/{repository}{'' if module == '.' else '/' + module}@{version}", "```", ""]
    return "\n".join(lines)
