from __future__ import annotations

import hashlib
import re

_NUMBER = re.compile(r"\b\d+(?:\.\d+)?\b")
_UUID = re.compile(
    r"\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b",
    re.IGNORECASE,
)
_EMAIL = re.compile(r"\b[\w.+-]+@[\w.-]+\.[a-zA-Z]{2,}\b")
_WHITESPACE = re.compile(r"\s+")


def normalize_prompt(prompt: str) -> str:
    lowered = prompt.lower()
    lowered = _EMAIL.sub("<email>", lowered)
    lowered = _UUID.sub("<uuid>", lowered)
    lowered = _NUMBER.sub("<num>", lowered)
    lowered = _WHITESPACE.sub(" ", lowered)
    return lowered.strip()


def fingerprint_prompt(prompt: str) -> str:
    normalized = normalize_prompt(prompt)
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()[:16]
