"""Synthetic-only catalog and privacy-guard tests; no volume runtime is exercised."""

import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import check_dynamic_host_volume_fixtures as fixture_guard
from dynamic_host_volume_test_support import DELETE, mutate_json

ROOT = Path(__file__).resolve().parents[1]
SENTINEL = "synthetic-rejected-sentinel"


class FixtureTests(unittest.TestCase):
    def setUp(self):
        self.guard = fixture_guard
        self.data = json.loads((ROOT / self.guard.FIXTURE_PATH / "cases.json").read_text())
        self.matrix = json.loads((ROOT / self.guard.MATRIX_PATH).read_text())

    def test_catalog_and_guard_exist(self):
        self.assertTrue((ROOT / "testdata/dynamic-host-volumes/cases.json").is_file(),
                        "synthetic dynamic-host-volume catalog is missing")
        self.assertTrue((ROOT / "ci/check_dynamic_host_volume_fixtures.py").is_file(),
                        "synthetic fixture guard is missing")
        self.guard.validate(self.data, self.matrix)
        self.assertTrue(all(test["status"] == "planned" for test in self.matrix["test_cases"]))

    def test_catalog_mutations(self):
        inputs = ("cases", 0, "inputs")
        volume = ("resources", "volumes", 0)
        mutations = [
            (("schema_version",), True), (("synthetic_only",), False),
            (("extra",), SENTINEL), (("resources",), DELETE),
            (("cases", 0, "extra"), SENTINEL), (("cases", 0, "facet"), DELETE),
            (("cases", 0, "id"), SENTINEL), (("cases", 1), self.data["cases"][0]),
            (("cases", 0, "expected", "extra"), SENTINEL),
            (("cases", 0, "expected", "status"), DELETE),
            (("cases", 2, "expected", "value"), "invented concrete outcome"),
            (("resources", "namespaces", 0), SENTINEL),
            (("resources", "nodes", 0, "pool"), SENTINEL),
            (("resources", "job", "task_group"), SENTINEL),
            (volume + ("node",), "node-b"),
            (volume + ("namespace",), "dhv-synthetic-b"),
            (volume + ("capacity_min_bytes",), 1048576.0),
            (inputs + ("marker",), DELETE), (inputs + ("extra",), SENTINEL),
        ]
        mutations.extend((volume + (field,), SENTINEL) for field in self.data["resources"]["volumes"][0])
        mutations.extend((("resources", "nodes", 0, field), SENTINEL)
                         for field in self.data["resources"]["nodes"][0])
        mutations.extend((inputs + (field,), SENTINEL) for field in self.data["cases"][0]["inputs"])
        mutations.extend((("cases", 0, field), value)
                         for field in ("behavior_id", "test_id", "facet", "scenario")
                         for value in (SENTINEL, "../cases.json", "/tmp/synthetic.json"))
        for path, value in mutations:
            with self.subTest(path=path):
                changed = mutate_json(self.data, path, value)
                with self.assertRaises(self.guard.GuardError):
                    self.guard.validate(changed, self.matrix)

    def test_dangling_matrix_links(self):
        for collection in ("behaviors", "test_cases"):
            with self.subTest(collection=collection):
                matrix = copy.deepcopy(self.matrix)
                matrix[collection] = [item for item in matrix[collection] if item["id"] not in
                                      {"DHV-B-003", "DHV-T-003"}]
                with self.assertRaises(self.guard.GuardError):
                    self.guard.validate(self.data, matrix)

    def prepare_tree(self, root):
        directory = root / self.guard.FIXTURE_PATH
        directory.mkdir(parents=True)
        (directory / "cases.json").write_text(json.dumps(self.data))
        matrix_path = root / self.guard.MATRIX_PATH
        matrix_path.parent.mkdir(parents=True)
        matrix_path.write_text(json.dumps(self.matrix))
        return directory

    def test_preflight_rejects_unknown_entries_and_symlinks_before_reads(self):
        for kind in ("unknown-file", "unknown-directory", "catalog-link", "extra-link", "directory-link", "parent-link", "matrix-link"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                directory = self.prepare_tree(root)
                target = root / "synthetic-target"
                target.write_text(SENTINEL)
                if kind == "unknown-file":
                    (directory / SENTINEL).write_text(SENTINEL)
                elif kind == "unknown-directory":
                    (directory / SENTINEL).mkdir()
                elif kind in {"catalog-link", "extra-link"}:
                    link = directory / ("cases.json" if kind == "catalog-link" else SENTINEL)
                    link.unlink(missing_ok=True)
                    link.symlink_to(target)
                elif kind == "matrix-link":
                    link = root / self.guard.MATRIX_PATH
                    link.unlink()
                    link.symlink_to(target)
                else:
                    link = directory if kind == "directory-link" else directory.parent
                    backing = root / "synthetic-backing"
                    link.rename(backing)
                    link.symlink_to(backing, target_is_directory=True)
                with patch.object(self.guard, "load_json") as read:
                    with self.assertRaises(self.guard.GuardError):
                        self.guard.check(root)
                    read.assert_not_called()

    def test_only_registered_content_is_read(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            directory = self.prepare_tree(root)
            (root / SENTINEL).write_text(SENTINEL)
            with patch.object(self.guard, "load_json", wraps=self.guard.load_json) as read:
                self.guard.check(root)
            self.assertEqual([call.args[0] for call in read.call_args_list],
                             [directory / "cases.json", root / self.guard.MATRIX_PATH])

    def test_file_size_bound(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            directory = self.prepare_tree(root)
            (directory / "cases.json").write_bytes(b" " * (self.guard.MAX_CATALOG_BYTES + 1))
            with self.assertRaises(self.guard.GuardError):
                self.guard.check(root)

    def test_cli_redacts_rejected_content_and_parser_errors(self):
        invalid = copy.deepcopy(self.data)
        invalid["cases"][0]["id"] = SENTINEL
        invalid_marker = copy.deepcopy(self.data)
        invalid_marker["cases"][0]["inputs"]["marker"] = SENTINEL
        raw_cases = (("{" + SENTINEL, False), (json.dumps(invalid), False),
                     (json.dumps(invalid_marker), False), (json.dumps(self.data), True),
                     ('{"schema_version":1,"schema_version":1,"' + SENTINEL + '":0}', False))
        for raw, unknown_file in raw_cases:
            with self.subTest(raw_kind=raw[:1]), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                directory = self.prepare_tree(root)
                (directory / "cases.json").write_text(raw)
                if unknown_file:
                    (directory / SENTINEL).write_text(SENTINEL)
                script = root / "ci/check_dynamic_host_volume_fixtures.py"
                script.parent.mkdir()
                script.write_bytes((ROOT / "ci/check_dynamic_host_volume_fixtures.py").read_bytes())
                result = subprocess.run([sys.executable, str(script)], capture_output=True, text=True, check=False)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, "")
                self.assertNotIn(SENTINEL, result.stderr)
                self.assertNotIn(str(root), result.stderr)
                tokens = result.stderr.split()
                self.assertEqual(len(tokens), 3)
                self.assertIn(tokens[0], self.guard.RULES)
                self.assertIn(tokens[1], {"catalog", *self.guard.CASE_SPECS})
                self.assertIn(tokens[2], self.guard.SAFE_FIELDS)


if __name__ == "__main__":
    unittest.main()
