"""Check that service acceptance cannot pass with missing, skipped or failed tests."""

import json
import unittest

import dynamodb_run


NAMES = ("TestDynamoDBLocalConditionalClaimsAndReplay",
         "TestDynamoDBLocalExpiryLeaseAndCompositeKeys",
         "TestDynamoDBLocalParameters")


class AcceptanceEvidenceTests(unittest.TestCase):
    def events(self, extra=()):
        return "\n".join(json.dumps(item) for item in
                         [*({"Action": "pass", "Test": name} for name in NAMES), *extra])

    def passed(self, stdout, exit_code=0):
        return all(item["passed"] for item in dynamodb_run.test_checks(stdout, exit_code))

    def test_complete_uncached_run_passes(self):
        self.assertTrue(self.passed(self.events()))

    def test_zero_exit_with_missing_tests_fails(self):
        for stdout in ("", '{"Action":"pass","Package":"fixture"}',
                       json.dumps({"Action": "pass", "Test": NAMES[0]})):
            with self.subTest(stdout=stdout):
                self.assertFalse(self.passed(stdout))

    def test_skipped_or_failed_child_blocks_parent_passes(self):
        for action in ("skip", "fail"):
            with self.subTest(action=action):
                self.assertFalse(self.passed(self.events([{"Action": action, "Test": NAMES[0] + "/child"}])))

    def test_failed_process_cannot_reuse_passed_events(self):
        self.assertFalse(self.passed(self.events(), 1))


if __name__ == "__main__":
    unittest.main(verbosity=2)
