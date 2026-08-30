"""Prove the inherited marketplace/payment QA tree is absent from the repository."""

import unittest
from pathlib import Path


class LegacyQARetirementTest(unittest.TestCase):
    def test_inherited_qa_tree_is_absent(self):
        repo_root = Path(__file__).resolve().parent.parent
        qa_root = repo_root / "qa"
        remaining = []
        if qa_root.exists() or qa_root.is_symlink():
            remaining.append(str(qa_root.relative_to(repo_root)))
            if qa_root.is_dir():
                for path in sorted(qa_root.rglob("*")):
                    remaining.append(str(path.relative_to(repo_root)))
        self.assertEqual(
            remaining,
            [],
            "inherited marketplace QA tree must be absent; remaining paths: %s"
            % ", ".join(remaining),
        )


if __name__ == "__main__":
    unittest.main()
