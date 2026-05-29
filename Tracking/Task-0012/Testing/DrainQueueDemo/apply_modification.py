"""Subagent edit applier for the drain-queue demonstration.

This is the *real work* a dispatched subagent performs against the target
program in its allocated worktree. It is deterministic and driven entirely by
the queued task's 'modification' spec so the diff is genuine, not hardcoded to
one task.

Usage:
    python apply_modification.py <worktree_dir> <task_spec_json_path>

It edits calc.py (and test_calc.py where the spec asks) inside <worktree_dir>,
then prints a short human-readable summary of what it changed. Exit code 0 on
success, non-zero on failure.
"""
import json
import re
import sys
from pathlib import Path


def _read(path):
    return path.read_text(encoding="utf-8")


def _write(path, text):
    # newline="" prevents Python's text-mode CRLF translation on Windows so the
    # file's original LF line endings are preserved and the diff stays minimal.
    with open(path, "w", encoding="utf-8", newline="") as fh:
        fh.write(text)


def add_operation(calc_path, test_path, mod):
    op_name = mod["op_name"]
    op_symbol = mod["op_symbol"]
    calc = _read(calc_path)

    if f'"{op_name}"' in calc and f"def {op_name}(" in calc:
        return f"operation '{op_name}' already present; no change"

    # Build the function body.
    if op_name == "div":
        func = (
            f"\ndef {op_name}(a, b):\n"
            f"    return a {op_symbol} b\n"
        )
    else:
        func = (
            f"\ndef {op_name}(a, b):\n"
            f"    return a {op_symbol} b\n"
        )

    # Insert the function just before the OPERATIONS dict.
    marker = "OPERATIONS = {"
    if marker not in calc:
        raise RuntimeError("could not find OPERATIONS dict in calc.py")
    calc = calc.replace(marker, func + "\n" + marker, 1)

    # Register in OPERATIONS.
    calc = calc.replace(
        '    "mul": mul,\n',
        f'    "mul": mul,\n    "{op_name}": {op_name},\n',
        1,
    )
    _write(calc_path, calc)

    # Add a test.
    test = _read(test_path)
    if op_name == "pow":
        new_test = (
            "\n    def test_pow(self):\n"
            "        self.assertEqual(calc.pow(2, 3), 8)\n"
        )
    elif op_name == "div":
        new_test = (
            "\n    def test_div(self):\n"
            "        self.assertEqual(calc.div(6, 2), 3)\n"
        )
    else:
        new_test = (
            f"\n    def test_{op_name}(self):\n"
            f"        self.assertIsNotNone(calc.{op_name}(2, 2))\n"
        )
    # Insert after the last existing test method (before the trailing __main__).
    anchor = '\nif __name__ == "__main__":'
    test = test.replace(anchor, new_test + anchor, 1)
    _write(test_path, test)
    return f"added operation '{op_name}' ({op_symbol}) and a covering test"


def set_version(calc_path, mod):
    new_version = mod["new_version"]
    calc = _read(calc_path)
    new_calc = re.sub(
        r'VERSION = "[^"]*"',
        f'VERSION = "{new_version}"',
        calc,
        count=1,
    )
    if new_calc == calc:
        raise RuntimeError("VERSION constant not found in calc.py")
    _write(calc_path, new_calc)
    return f"set VERSION to {new_version}"


def add_version_flag(calc_path, mod):
    calc = _read(calc_path)
    if "--version" in calc:
        return "--version flag already present; no change"
    anchor = "    if len(argv) != 3:\n"
    if anchor not in calc:
        raise RuntimeError("could not find argv length check in calc.py")
    flag_block = (
        '    if len(argv) == 1 and argv[0] == "--version":\n'
        "        print(VERSION)\n"
        "        return 0\n"
    )
    calc = calc.replace(anchor, flag_block + anchor, 1)
    _write(calc_path, calc)
    return "added --version flag handling"


def main(argv):
    if len(argv) != 2:
        print("usage: python apply_modification.py <worktree_dir> <task_spec_json>")
        return 2
    worktree_dir = Path(argv[0])
    spec = json.loads(Path(argv[1]).read_text(encoding="utf-8"))
    mod = spec["modification"]
    kind = mod["kind"]

    calc_path = worktree_dir / "calc.py"
    test_path = worktree_dir / "test_calc.py"

    if kind == "add_operation":
        summary = add_operation(calc_path, test_path, mod)
    elif kind == "set_version":
        summary = set_version(calc_path, mod)
    elif kind == "add_version_flag":
        summary = add_version_flag(calc_path, mod)
    else:
        print(f"unknown modification kind: {kind}")
        return 2

    print(summary)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
