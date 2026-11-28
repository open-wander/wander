import importlib.util
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location("check_gosec_changed", Path(__file__).with_name("check_gosec_changed.py"))
gate = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(gate)

DIFF = """diff --git a/a/x.go b/a/x.go
--- a/a/x.go
+++ b/a/x.go
@@ -10,0 +11,3 @@ func f() {
+one
+two
+three
@@ -40 +43 @@ func g() {
-old
+new
@@ -50,2 +53,0 @@ func h() {
-gone
-gone
diff --git a/b/y.go b/b/y.go
deleted file mode 100644
--- a/b/y.go
+++ /dev/null
@@ -1 +0,0 @@
-x
"""


class ParseDiff(unittest.TestCase):
    def test_added_and_changed_lines(self):
        self.assertEqual(gate.parse_diff(DIFF), {"a/x.go": {11, 12, 13, 43}})

    def test_pure_deletion_adds_nothing(self):
        self.assertNotIn(53, gate.parse_diff(DIFF)["a/x.go"])


class NewFindings(unittest.TestCase):
    changed = {"a/x.go": {11, 12, 13, 43}}

    def issue(self, line, path="/r/a/x.go"):
        return {"file": path, "line": line, "rule_id": "G304", "severity": "MEDIUM", "details": "d"}

    def test_finding_on_changed_line(self):
        self.assertEqual(len(gate.new_findings([self.issue("12")], self.changed, "/r")), 1)

    def test_finding_on_unchanged_line(self):
        self.assertEqual(gate.new_findings([self.issue("20")], self.changed, "/r"), [])

    def test_range_overlapping_a_changed_line(self):
        self.assertEqual(len(gate.new_findings([self.issue("40-43")], self.changed, "/r")), 1)

    def test_range_outside_changed_lines(self):
        self.assertEqual(gate.new_findings([self.issue("14-42")], self.changed, "/r"), [])

    def test_unchanged_file(self):
        self.assertEqual(gate.new_findings([self.issue("12", "/r/c/z.go")], self.changed, "/r"), [])


if __name__ == "__main__":
    unittest.main()
