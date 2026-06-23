from __future__ import annotations

import json
from pathlib import Path
from typing import Iterable

from miser.models import LLMCall


def load_jsonl(path: str | Path) -> list[LLMCall]:
    calls: list[LLMCall] = []
    with Path(path).open("r", encoding="utf-8") as handle:
        for line_number, line in enumerate(handle, start=1):
            stripped = line.strip()
            if not stripped:
                continue
            try:
                row = json.loads(stripped)
                calls.append(LLMCall.from_dict(row))
            except Exception as exc:
                raise ValueError(f"failed to parse {path}:{line_number}: {exc}") from exc
    return calls


def dumps_receipts(receipts: Iterable[object]) -> str:
    return json.dumps([receipt.to_dict() for receipt in receipts], indent=2)
