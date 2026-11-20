"""Bounded offline checks for the registered synthetic dynamic-volume catalog.

No private-artifact denylist is read. Diagnostic text never includes input values.
"""
import json
import os
from pathlib import Path
import stat
import sys

ROOT = Path(__file__).resolve().parents[1]
FIXTURE_PATH = Path("testdata/dynamic-host-volumes")
MATRIX_PATH = Path("docs/compatibility/dynamic-host-volumes.json")
MAX_CATALOG_BYTES = 65536
MAX_MATRIX_BYTES = 2 * 1024 * 1024
NAMESPACES = ("dhv-synthetic-a", "dhv-synthetic-b")
NODES = ("node-a", "node-b")
JOB = {"id": "job-synthetic-a", "task_group": "group-synthetic-a"}
VOLUMES = [
    {"id": identifier, "name": name, "namespace": NAMESPACES[index], "node": NODES[index],
     "host_path": "/srv/wander-synthetic/" + name,
     "capacity_min_bytes": minimum, "capacity_max_bytes": maximum,
     "plugin_id": "mkdir", "parameters": {"mode": mode}, "marker": "wander-synthetic-marker-" + suffix}
    for index, (identifier, name, minimum, maximum, mode, suffix) in enumerate((
        ("11111111-1111-4111-8111-111111111111", "data-a", 1048576, 2097152, "0700", "a"),
        ("22222222-2222-4222-8222-222222222222", "data-b", 2097152, 4194304, "0750", "b"),
    ))
]
RESOURCES = {
    "namespaces": list(NAMESPACES),
    "nodes": [
        {"id": "00000000-0000-4000-8000-000000000001", "name": NODES[0], "pool": "pool-a"},
        {"id": "00000000-0000-4000-8000-000000000002", "name": NODES[1], "pool": "pool-b"},
    ],
    "volumes": VOLUMES,
    "job": JOB,
}
# IDs are fixed and append-only. These define scenarios, not runtime expectations.
CASE_SPECS = {
    "DHV-F-001": ("create", "DHV-B-003", "DHV-T-003", "contract", 0),
    "DHV-F-002": ("register-existing", "DHV-B-004", "DHV-T-004", "contract", 1),
    "DHV-F-003": ("namespace-isolation", "DHV-B-012", "DHV-T-012", "outputs", 0),
    "DHV-F-004": ("missing-inputs", "DHV-B-018", "DHV-T-018", "errors", 0),
    "DHV-F-005": ("incompatible-ranges", "DHV-B-112", "DHV-T-112", "errors", 0),
    "DHV-F-006": ("plugin-failure", "DHV-B-096", "DHV-T-096", "errors", 0),
    "DHV-F-007": ("delete-in-use", "DHV-B-022", "DHV-T-022", "contract", 0),
    "DHV-F-008": ("registered-path-restriction", "DHV-B-110", "DHV-T-110", "lifecycle", 1),
}
SAFE_FIELDS = {"catalog", "resources", "cases", "id", "scenario", "behavior_id", "test_id",
               "facet", "inputs", "expected", "filesystem", "json", "matrix", "schema"}
RULES = {"DHV-G-SCHEMA", "DHV-G-VOCABULARY", "DHV-G-REFERENCE", "DHV-G-OUTCOME",
         "DHV-G-FILESYSTEM", "DHV-G-JSON", "DHV-G-INTERNAL"}


class GuardError(ValueError):
    def __init__(self, rule, fixture="catalog", field="catalog"):
        self.rule = rule if rule in RULES else "DHV-G-INTERNAL"
        self.fixture = fixture if fixture in CASE_SPECS else "catalog"
        self.field = field if field in SAFE_FIELDS else "catalog"
        super().__init__(self.rule + " " + self.fixture + " " + self.field)


def require(condition, rule, fixture="catalog", field="catalog"):
    if not condition:
        raise GuardError(rule, fixture, field)


def closed(obj, fields, fixture="catalog", field="schema"):
    require(isinstance(obj, dict) and obj.keys() == set(fields), "DHV-G-SCHEMA", fixture, field)


def same_json(left, right):
    return json.dumps(left, sort_keys=True) == json.dumps(right, sort_keys=True)


def synthetic_inputs(fixture):
    """Return the closed scenario vocabulary, including deliberate invalid inputs."""
    volume = VOLUMES[CASE_SPECS[fixture][4]]
    inputs = {field: volume[field] for field in
              ("namespace", "node", "capacity_min_bytes", "capacity_max_bytes", "parameters", "marker")}
    inputs.update(volume=volume["name"], job=JOB["id"], task_group=JOB["task_group"], plugin_result="not-run")
    if fixture == "DHV-F-003":
        inputs["namespace"] = NAMESPACES[1]
    elif fixture == "DHV-F-004":
        inputs["node"] = None
    elif fixture == "DHV-F-005":
        inputs["capacity_min_bytes"], inputs["capacity_max_bytes"] = 2097152, 1048576
    elif fixture == "DHV-F-006":
        inputs["plugin_result"] = "synthetic-failure"
    return inputs


def validate(data, matrix):
    closed(data, {"schema_version", "synthetic_only", "resources", "cases"})
    require(type(data["schema_version"]) is int and data["schema_version"] == 1
            and data["synthetic_only"] is True, "DHV-G-SCHEMA")
    require(same_json(data["resources"], RESOURCES), "DHV-G-VOCABULARY", field="resources")
    cases = data["cases"]
    require(isinstance(cases, list) and len(cases) == len(CASE_SPECS), "DHV-G-SCHEMA", field="cases")
    require(isinstance(matrix, dict) and isinstance(matrix.get("behaviors"), list)
            and isinstance(matrix.get("test_cases"), list), "DHV-G-REFERENCE", field="matrix")
    behaviors = {b["id"]: b for b in matrix["behaviors"]}
    tests = {t["id"]: t for t in matrix["test_cases"]}
    seen = set()
    for case in cases:
        closed(case, {"id", "scenario", "behavior_id", "test_id", "facet", "inputs", "expected"})
        fixture = case["id"]
        require(isinstance(fixture, str) and fixture in CASE_SPECS and fixture not in seen,
                "DHV-G-REFERENCE", field="id")
        seen.add(fixture)
        scenario, behavior_id, test_id, facet, _ = CASE_SPECS[fixture]
        for field, expected in (("scenario", scenario), ("behavior_id", behavior_id),
                                ("test_id", test_id), ("facet", facet)):
            require(case[field] == expected, "DHV-G-REFERENCE", fixture, field)
        require(behavior_id in behaviors and test_id in tests, "DHV-G-REFERENCE", fixture, "matrix")
        behavior = behaviors[behavior_id]
        require(facet in behavior["assertions"], "DHV-G-REFERENCE", fixture, "facet")
        assertion = behavior["assertions"][facet]
        require(test_id in assertion["test_ids"] and tests[test_id]["synthetic"] is True,
                "DHV-G-REFERENCE", fixture, "test_id")
        closed(case["inputs"], synthetic_inputs(fixture), fixture, "inputs")
        require(same_json(case["inputs"], synthetic_inputs(fixture)), "DHV-G-VOCABULARY", fixture, "inputs")
        closed(case["expected"], {"status", "value"}, fixture, "expected")
        # No invented error/status/output is permitted, including on negative cases.
        require(case["expected"] == {"status": assertion["status"], "value": assertion["value"]},
                "DHV-G-OUTCOME", fixture, "expected")
        if assertion["status"] == "unresolved":
            require(case["expected"]["value"] is None, "DHV-G-OUTCOME", fixture, "expected")


def reject_symlinks(path):
    require(not any(parent.is_symlink() for parent in (path, *path.parents)), "DHV-G-FILESYSTEM",
            field="filesystem")


def load_json(path, limit, field):
    reject_symlinks(path)
    try:
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(descriptor, "rb") as stream:
            info = os.fstat(stream.fileno())
            require(stat.S_ISREG(info.st_mode) and info.st_size <= limit, "DHV-G-FILESYSTEM", field=field)
            raw = stream.read(limit + 1)
        require(len(raw) <= limit, "DHV-G-FILESYSTEM", field=field)
        return json.loads(raw, object_pairs_hook=unique_keys)
    except (OSError, UnicodeError, ValueError, RecursionError) as error:
        if isinstance(error, GuardError):
            raise
        raise GuardError("DHV-G-JSON", field=field) from None


def unique_keys(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "DHV-G-SCHEMA", field="json")
        result[key] = value
    return result


def check(root=ROOT):
    directory = root / FIXTURE_PATH
    reject_symlinks(directory)
    # Inspect every entry before reading any fixture or matrix content.
    try:
        names = set()
        with os.scandir(directory) as entries:
            for entry in entries:
                require(entry.name == "cases.json" and not entry.is_symlink()
                        and entry.is_file(follow_symlinks=False), "DHV-G-FILESYSTEM", field="filesystem")
                names.add(entry.name)
        require(names == {"cases.json"}, "DHV-G-FILESYSTEM", field="filesystem")
    except OSError:
        raise GuardError("DHV-G-FILESYSTEM", field="filesystem") from None
    reject_symlinks(root / MATRIX_PATH)
    data = load_json(directory / "cases.json", MAX_CATALOG_BYTES, "json")
    matrix = load_json(root / MATRIX_PATH, MAX_MATRIX_BYTES, "matrix")
    validate(data, matrix)


def main():
    try:
        require(len(sys.argv) == 1, "DHV-G-SCHEMA")
        check()
    except GuardError as error:
        print(str(error), file=sys.stderr)
        return 1
    except Exception:
        # Unexpected failures must not expose parser excerpts, paths, or values.
        print("DHV-G-INTERNAL catalog catalog", file=sys.stderr)
        return 1
    print("DHV-G-OK catalog catalog")
    return 0


if __name__ == "__main__":
    sys.exit(main())
