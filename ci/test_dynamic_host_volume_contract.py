"""Offline contract checks; these do not test a volume implementation."""

import copy
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
MATRIX = ROOT / "docs/compatibility/dynamic-host-volumes.json"
DELETE = object()


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.assertTrue(MATRIX.is_file(), "public dynamic-host-volume matrix is missing")
        spec = importlib.util.spec_from_file_location(
            "contract", ROOT / "ci/validate_dynamic_host_volume_contract.py"
        )
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.data = json.loads(MATRIX.read_text())

    def test_contract_exists_and_validates(self):
        self.module.validate(self.data, ROOT)
        self.assertTrue((MATRIX.parent / "dynamic-host-volumes-provenance.md").is_file())

    def test_implemented_test_location_is_accepted(self):
        changed = copy.deepcopy(self.data)
        changed["test_cases"][0].update(
            status="implemented", path="ci/test_dynamic_host_volume_contract.py",
            name="test_contract_exists_and_validates",
        )
        self.module.validate(changed, ROOT)

    def observation_fixture(self, anchor="requirements"):
        changed = copy.deepcopy(self.data)
        source = changed["sources"][0]
        source.update(category="black_box_observation", locator=(
            "docs/compatibility/dynamic-host-volumes-provenance.md#" + anchor))
        for behavior in changed["behaviors"]:
            for assertion in behavior["assertions"].values():
                if source["id"] in assertion["evidence_refs"] and assertion["status"] == "documented":
                    assertion["status"] = "observed"
        return changed

    def test_observation_requires_its_own_record(self):
        for anchor in ("requirements", "evidence-rules"):
            with self.subTest(anchor=anchor), self.assertRaises(ValueError):
                self.module.validate(self.observation_fixture(anchor), ROOT)

    def test_observation_record_metadata(self):
        anchor = "observation-synthetic-a"
        data = self.observation_fixture(anchor)
        record = {
            "source_id": data["sources"][0]["id"], "binary_version": "synthetic-test-binary",
            "synthetic": True, "request": "GET synthetic volume data-a",
            "response": "Synthetic response for data-a", "date": data["evidence_cutoff"],
            "limitations": "Validator fixture only; no binary was executed.",
        }
        original_read = Path.read_text

        def validate_record(value):
            text = original_read(MATRIX.parent / "dynamic-host-volumes-provenance.md")
            text += "\n## Synthetic observation {#" + anchor + "}\n\n```json\n" + json.dumps(value) + "\n```\n"

            def read(path, *args, **kwargs):
                if path.name == "dynamic-host-volumes-provenance.md":
                    return text
                return original_read(path, *args, **kwargs)

            with patch.object(Path, "read_text", read):
                self.module.validate(data, ROOT)

        validate_record(record)
        invalid_records = [None] + [
            {name: value for name, value in record.items() if name != key} for key in record
        ]
        invalid_records.extend(dict(record, **{key: value}) for key, value in (
            ("source_id", "different-source"), ("synthetic", False), ("date", "2026-11-07"),
            ("binary_version", ""), ("request", ""), ("response", ""), ("limitations", ""),
        ))
        for number, value in enumerate(invalid_records):
            with self.subTest(invalid_record=number), self.assertRaises(ValueError):
                validate_record(value)

    def test_invalid_mutations(self):
        assertion = ("behaviors", 0, "assertions")
        source = ("sources", 0)
        test = ("test_cases", 0)
        cases = [
            ("missing reference", assertion + ("contract", "evidence_refs"), []),
            ("dangling evidence", assertion + ("contract", "evidence_refs"), ["missing"]),
            ("dangling test", assertion + ("contract", "test_ids"), ["missing"]),
            ("missing ACL mapping", assertion + ("auth",), DELETE),
            ("missing error mapping", assertion + ("errors",), DELETE),
            ("invented unresolved value", assertion + ("errors", "value"), "invented"),
            ("unresolved implemented", ("behaviors", 0, "implementation_status"), "implemented"),
            ("unresolved verified", ("behaviors", 0, "implementation_status"), "verified"),
            ("missing questions", ("behaviors", 0, "unresolved_questions"), []),
            ("unknown story", ("behaviors", 0, "prd_refs"), ["US-999"]),
            ("unknown requirement", ("behaviors", 0, "prd_refs"), ["FR-999"]),
            ("observation without evidence", assertion + ("contract", "status"), "observed"),
            ("invalid version", ("schema_version",), 2),
            ("privacy declaration", ("synthetic_only",), False),
            ("extra schema key", ("unexpected",), True),
            ("invalid date", source + ("date",), "2026-99-99"),
            ("future evidence", source + ("date",), "2027-01-01"),
            ("missing scope", source + ("scope",), ""),
            ("implemented without location", test + ("status",), "implemented"),
            ("planned with location", test + ("path",), "ci/test_dynamic_host_volume_contract.py"),
            ("missing surface", ("behaviors",),
             [b for b in self.data["behaviors"] if b["surface"] != "ui"]),
        ]
        for collection in ("sources", "test_cases", "behaviors"):
            cases.append(("duplicate " + collection, (collection, 1, "id"),
                          self.data[collection][0]["id"]))
        for collection in ("sources", "test_cases"):
            cases.append(("orphan " + collection, (collection,),
                          self.data[collection] + [dict(self.data[collection][0], id="orphan")]))
        categories = ("source", "tests", "patch", "commit", "repository",
                      "generated_asset", "private", "implementation")
        cases.extend(("prohibited " + category, source + ("category",), category)
                     for category in categories)
        urls = (
            "https://github.com/example/example/blob/main/example.go",
            "https://developer.hashicorp.com/nomad/commit/example",
            "https://developer.hashicorp.com/nomad/docs/configuration/client.patch",
            "https://developer.hashicorp.com/nomad/docs/generated/example.js",
            "https://example.invalid/nomad/api-docs/volumes",
            "https://developer.hashicorp.com.evil.invalid/nomad/api-docs/volumes",
            "http://developer.hashicorp.com/nomad/api-docs/volumes",
            "https://developer.hashicorp.com/nomad/api-docs/volumes?raw=1",
        )
        cases.extend(("prohibited URL " + url, source + ("locator",), url) for url in urls)
        for path, name in (("../outside-test.py", "test_example"),
                           ("ci/test_dynamic_host_volume_contract.py", "absent_test_name")):
            cases.append(("invalid implemented location " + path, test,
                          dict(self.data["test_cases"][0], status="implemented",
                               path=path, name=name)))
        for label, path, value in cases:
            with self.subTest(mutation=label):
                changed = copy.deepcopy(self.data)
                parent = changed
                for key in path[:-1]:
                    parent = parent[key]
                if value is DELETE:
                    del parent[path[-1]]
                else:
                    parent[path[-1]] = value
                with self.assertRaises(ValueError):
                    self.module.validate(changed, ROOT)


if __name__ == "__main__":
    unittest.main()
