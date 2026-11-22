"""Validate the public evidence ledger without network access or third-party packages."""

from datetime import date
import json
from pathlib import Path
import re
import sys
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[1]
MATRIX = ROOT / "docs/compatibility/dynamic-host-volumes.json"
PRD = Path("prds/active/prd-dynamic-host-volumes.md")
SURFACES = {"http_api", "go_client", "cli", "client_config", "plugin_contract",
            "builtin_plugin", "job_fields", "acls", "claims", "scheduling_lifecycle", "ui"}
FACETS = {"contract", "inputs_defaults", "outputs", "errors", "auth", "concurrency", "lifecycle"}
DOC_PATHS = {
    "/nomad/api-docs/volumes", "/nomad/docs/other-specifications/volume/host",
    "/nomad/plugins/author/host-volume", "/nomad/docs/job-specification/volume",
    "/nomad/docs/configuration/client", "/nomad/commands/volume",
    "/nomad/docs/secure/acl/policies", "/nomad/docs/other-specifications/acl-policy",
}
CATEGORIES = {"official_documentation", "black_box_observation", "wander_requirement"}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def keys(obj, required, optional=()):
    require(isinstance(obj, dict), "expected object")
    require(set(required) <= obj.keys() <= set(required) | set(optional), "schema keys")


def nonempty(value):
    return isinstance(value, str) and bool(value.strip())


def dated(value):
    require(isinstance(value, str), "date must be a string")
    require(date.fromisoformat(value).isoformat() == value, "invalid date")


def index(items):
    require(isinstance(items, list) and items, "expected nonempty collection")
    result = {}
    for item in items:
        require(isinstance(item, dict) and nonempty(item.get("id")), "missing ID")
        require(item["id"] not in result, "duplicate ID")
        result[item["id"]] = item
    return result


def validate_observation(source, anchor, text):
    require(re.fullmatch(r"observation-[a-z0-9][a-z0-9-]*", anchor), "observation-specific anchor required")
    headings = list(re.finditer(r"(?m)^#{2,6}[ \t]+[^\n]*\{#" + re.escape(anchor)
                               + r"\}[ \t]*$", text))
    require(len(headings) == 1, "unique observation heading required")
    section = re.split(r"(?m)^#{1,6}[ \t]+", text[headings[0].end():], maxsplit=1)[0]
    records = re.findall(r"(?ms)^```json[ \t]*\n(.*?)^```[ \t]*$", section)
    require(len(records) == 1, "observation JSON record required")
    record = json.loads(records[0])
    keys(record, {"source_id", "binary_version", "synthetic", "request", "response", "date", "limitations"})
    require(record["source_id"] == source["id"] and record["synthetic"] is True, "observation identity/privacy")
    require(all(nonempty(record[key]) for key in ("binary_version", "request", "response", "limitations")),
            "observation metadata missing")
    dated(record["date"])
    require(record["date"] == source["date"], "observation date mismatch")


def validate(data, root=ROOT):
    keys(data, {"schema_version", "feature", "evidence_cutoff", "synthetic_only",
                "sources", "test_cases", "behaviors"})
    require(type(data["schema_version"]) is int and data["schema_version"] == 1, "schema version")
    require(data["feature"] == "dynamic-host-volumes" and data["synthetic_only"] is True, "feature/privacy policy")
    dated(data["evidence_cutoff"])
    sources, tests, behaviors = (index(data[name]) for name in ("sources", "test_cases", "behaviors"))
    all_ids = list(sources) + list(tests) + list(behaviors)
    require(len(all_ids) == len(set(all_ids)), "cross-collection duplicate ID")
    prd = (root / PRD).read_text()
    prd_refs = set(re.findall(r"(?m)^(?:#{1,6}[ \t]+|-[ \t]+\*\*)(US-\d{3}|FR-\d+):", prd))
    prd_refs.update(re.findall(r"(?m)^#{1,6}[ \t]+(\d+(?:\.\d+)+)\b", prd))
    require(prd_refs, "PRD identifiers missing")
    for source in sources.values():
        keys(source, {"id", "category", "locator", "date", "scope", "summary", "limitations"})
        require(source["category"] in CATEGORIES, "prohibited source category")
        dated(source["date"])
        require(source["date"] <= data["evidence_cutoff"], "source after cutoff")
        require(all(nonempty(source[k]) for k in ("locator", "scope", "summary", "limitations")), "source detail missing")
        locator = source["locator"]
        if source["category"] == "official_documentation":
            url = urlsplit(locator)
            require(url.scheme == "https" and url.netloc == "developer.hashicorp.com"
                    and url.path in DOC_PATHS and not url.query
                    and re.fullmatch(r"[a-z0-9-]*", url.fragment), "prohibited documentation URL")
        else:
            path, sep, anchor = locator.partition("#")
            require(sep and nonempty(anchor), "local evidence anchor required")
            require(path == "docs/compatibility/dynamic-host-volumes-provenance.md", "prohibited evidence path")
            text = (root / path).read_text()
            require(f"{{#{anchor}}}" in text, "missing evidence anchor")
            if source["category"] == "black_box_observation":
                validate_observation(source, anchor, text)
    for test in tests.values():
        keys(test, {"id", "scenario", "expected_outcome", "status", "synthetic"}, {"path", "name"})
        require(all(nonempty(test[k]) for k in ("scenario", "expected_outcome")), "test description missing")
        require(test["synthetic"] is True and test["status"] in {"planned", "implemented"}, "test status/privacy")
        if test["status"] == "implemented":
            require(nonempty(test.get("path")) and nonempty(test.get("name")), "implemented test path/name required")
            require(re.fullmatch(r"(?:test_[A-Za-z0-9_]+|Test[A-Za-z0-9_]+)", test["name"]), "expected test function name")
            path = (root / test["path"]).resolve()
            require(path.is_relative_to(root.resolve()) and path.is_file(), "implemented test path invalid")
            require(re.search(r"(?m)^\s*(?:async\s+def|def|func)\s+" + re.escape(test["name"])
                              + r"\s*\(", path.read_text()), "implemented test definition missing")
        else:
            require("path" not in test and "name" not in test, "planned test has implementation location")
    used_sources, used_tests, surfaces = set(), set(), set()
    for behavior in behaviors.values():
        keys(behavior, {"id", "surface", "operation_field", "prd_refs", "implementation_status", "assertions", "unresolved_questions"})
        surfaces.add(behavior["surface"])
        require(behavior["surface"] in SURFACES and nonempty(behavior["operation_field"]), "invalid surface/operation")
        require(isinstance(behavior["prd_refs"], list) and behavior["prd_refs"] and all(
            isinstance(ref, str) and ref in prd_refs
            for ref in behavior["prd_refs"]), "PRD references missing or unknown")
        require(behavior["implementation_status"] in {"planned", "unresolved", "implemented", "verified"}, "implementation status")
        require(isinstance(behavior["unresolved_questions"], list) and all(map(nonempty, behavior["unresolved_questions"])), "question schema")
        keys(behavior["assertions"], FACETS)
        unresolved = False
        for assertion in behavior["assertions"].values():
            keys(assertion, {"status", "value", "evidence_refs", "test_ids"}, {"implementation"})
            status = assertion["status"]
            require(status in {"documented", "observed", "requirement", "unresolved"}, "assertion status")
            require(isinstance(assertion["evidence_refs"], list) and assertion["evidence_refs"], "missing evidence reference")
            require(isinstance(assertion["test_ids"], list) and assertion["test_ids"], "missing synthetic test")
            require(all(ref in sources for ref in assertion["evidence_refs"]), "dangling evidence reference")
            require(all(ref in tests for ref in assertion["test_ids"]), "dangling test reference")
            if "implementation" in assertion:
                require(assertion["implementation"] in ("implemented", "verified"), "assertion implementation status")
                require(status != "unresolved", "unresolved assertion cannot be implemented")
                require(all(tests[ref]["status"] == "implemented" for ref in assertion["test_ids"]),
                        "implemented assertion needs implemented tests")
            used_sources.update(assertion["evidence_refs"])
            used_tests.update(assertion["test_ids"])
            category = {"documented": "official_documentation", "observed": "black_box_observation",
                        "requirement": "wander_requirement"}.get(status)
            if category:
                require(any(sources[ref]["category"] == category for ref in assertion["evidence_refs"]), "evidence category mismatch")
                require(nonempty(assertion["value"]), "assertion value missing")
            else:
                unresolved = True
                require(assertion["value"] is None, "unresolved assertion cannot state behavior")
            if behavior["implementation_status"] in {"implemented", "verified"}:
                require(all(tests[ref]["status"] == "implemented" for ref in assertion["test_ids"]), "implemented behavior needs implemented tests")
        if unresolved:
            require(behavior["implementation_status"] in {"planned", "unresolved"}
                    and behavior["unresolved_questions"], "unresolved assertions cannot be implemented/verified")
    require(surfaces == SURFACES, "required surface coverage missing")
    require(used_sources == sources.keys() and used_tests == tests.keys(), "orphan evidence/test")


if __name__ == "__main__":
    try:
        validate(json.loads(MATRIX.read_text()))
    except (ValueError, OSError, TypeError, KeyError) as error:
        sys.exit(f"dynamic host volume contract invalid: {error}")
    print("dynamic host volume contract valid (offline; no runtime compatibility claim)")
