#!/usr/bin/env python3
"""Pure tests plus optional real NATS test against a disposable local server.

PANDA_WIKI_TEST_RAGLITE_CANCEL_NATS=1 enables the integration test. It only
connects to 127.0.0.1:14223 with fixed disposable-test credentials. No production.
"""

import base64
import contextlib
from datetime import datetime, timedelta, timezone
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("backlog", Path(__file__).parents[1] / "inspect-raglite-backlog.py")
backlog = importlib.util.module_from_spec(spec)
spec.loader.exec_module(backlog)
DATASET = "f5cf4b10-4c82-4d99-87d0-6d393c25ec91"
OTHER = "b5cf4b10-4c82-4d99-87d0-6d393c25ec91"


def payload(doc="doc-a", dataset=DATASET, task="", kind="document_indexing"):
    return {"type": kind, "dataset_id": dataset, "document_id": doc, "task_id": task}


def message(sequence=11, **overrides):
    result = {"seq": sequence, "subject": "raglite.tasks.document_indexing",
              "time": "2026-10-30T01:18:02.557108074Z",
              "data": base64.b64encode(json.dumps(payload()).encode()).decode()}
    result.update(overrides)
    return result


def options(**overrides):
    result = dict(dataset_id=DATASET, through_seq=20,
                  published_before="2026-10-30T01:18:03Z", apply=False,
                  nats_container="unused", raglite_container="unused", backup_dir="unused")
    result.update(overrides)
    return SimpleNamespace(**result)


class ScopeTests(unittest.TestCase):
    def test_scope_protects_delivered_future_time_and_new_sequence(self):
        self.assertTrue(backlog.target_message(message(), 10, options()))
        self.assertFalse(backlog.target_message(message(sequence=10), 10, options()))
        self.assertFalse(backlog.target_message(message(sequence=21), 10, options()))
        self.assertFalse(backlog.target_message(message(time="2026-10-31T00:00:00Z"), 10, options()))

    def test_other_dataset_managed_task_unknown_type_and_invalid_json_protected(self):
        for item in (payload(dataset=OTHER), payload(task="managed-task"),
                     payload(kind="other"), {}, [], "invalid"):
            with self.subTest(item=item):
                data = base64.b64encode(json.dumps(item).encode()).decode()
                self.assertFalse(backlog.target_message(message(data=data), 10, options()))
        self.assertFalse(backlog.target_message(message(data="not base64"), 10, options()))
        self.assertFalse(backlog.target_message(message(subject="raglite.tasks.other"), 10, options()))

    def test_nanosecond_time_and_required_timezone(self):
        self.assertEqual(backlog.parse_time("2026-10-30T01:18:02.557108074Z").microsecond, 557108)
        with self.assertRaises(RuntimeError):
            backlog.parse_time("2026-10-30T01:18:02")

    def test_schema_summary_does_not_expose_body_or_credentials(self):
        data = payload()
        data.update(content="PRIVATE_BODY", api_key="PRIVATE_KEY",
                    metadata={"filename": "PRIVATE_FILENAME"})
        report = backlog.describe_payload(base64.b64encode(json.dumps(data).encode()).decode())
        for secret in ("PRIVATE_BODY", "PRIVATE_KEY", "PRIVATE_FILENAME"):
            self.assertNotIn(secret, json.dumps(report))


@unittest.skipUnless(os.getenv("PANDA_WIKI_TEST_RAGLITE_CANCEL_NATS") == "1", "disposable NATS not enabled")
class RealNatsTest(unittest.TestCase):
    def test_preview_partial_failure_retry_and_preserved_messages(self):
        def connect(unused=None):
            return backlog.NatsReader("127.0.0.1", 14223, "test-reader", "isolated-test-only")
        client = connect()
        self.addCleanup(client.close)
        # This is a disposable server with no volumes and fixed test credentials.
        for subject in ("$JS.API.STREAM.CREATE.raglite_tasks",):
            reply = client._request(subject, {"name": "raglite_tasks", "subjects": ["raglite.tasks.>"],
                                              "retention": "workqueue", "storage": "memory"})
            self.assertNotIn("error", reply)
        jobs = [payload(doc="inflight"), payload(doc="already-succeeded"),
                payload(doc="already-succeeded"), payload(dataset=OTHER),
                payload(task="managed-new"), payload(kind="other"),
                payload(doc="pending"), payload(doc="after-cutoff")]
        for index, job in enumerate(jobs):
            subject = "raglite.tasks.other" if index == 5 else "raglite.tasks.document_indexing"
            self.assertEqual(client._request(subject, job)["seq"], index + 1)
        reply = client._request("$JS.API.CONSUMER.CREATE.raglite_tasks.raglite_workers", {
            "stream_name": "raglite_tasks", "config": {"durable_name": "raglite_workers",
            "deliver_policy": "all", "ack_policy": "explicit", "filter_subject": "raglite.tasks.*"}})
        self.assertNotIn("error", reply)
        received = client._request("$JS.API.CONSUMER.MSG.NEXT.raglite_tasks.raglite_workers", {"batch": 1})
        self.assertEqual(received["document_id"], "inflight")
        before = backlog.api(client, backlog.CONSUMER_INFO)
        self.assertEqual(before["num_ack_pending"], 1)
        self.assertEqual(before["num_pending"], 7)
        with tempfile.TemporaryDirectory() as directory:
            args = options(through_seq=7, published_before=(datetime.now(timezone.utc) + timedelta(days=1)).isoformat(),
                           backup_dir=directory)
            states = {"already-succeeded": {"status": "SUCCEEDED"}, "pending": {"status": "PENDING"}}
            with patch.object(backlog, "document_states", return_value=states), \
                    patch.object(backlog, "connect_nats", side_effect=connect):
                output = io.StringIO()
                with contextlib.redirect_stdout(output):
                    backlog.cancel_old(client, args)
                self.assertIn('"匹配的未领取旧任务": 3', output.getvalue())
                self.assertIn('"对应不同文档数": 2', output.getvalue())
                self.assertEqual(list(Path(directory).iterdir()), [])
                self.assertEqual(backlog.api(client, backlog.CONSUMER_INFO)["num_pending"], 7)
                args.apply = True
                real_request = client._request
                deletes = []

                def fail_second_delete(subject, data=None):
                    if subject == backlog.MESSAGE_DELETE:
                        # Every delete must have a complete, durable backup first.
                        backups = list(Path(directory).glob("*/messages.jsonl"))
                        self.assertEqual(len(backups), 1)
                        self.assertEqual(len(backups[0].read_text().splitlines()), 8)
                        self.assertEqual(backups[0].stat().st_mode & 0o777, 0o600)
                        if deletes:
                            raise RuntimeError("injected failure")
                        deletes.append(data["seq"])
                    return real_request(subject, data)

                with patch.object(client, "_request", side_effect=fail_second_delete), \
                        contextlib.redirect_stdout(io.StringIO()):
                    with self.assertRaisesRegex(RuntimeError, "injected failure"):
                        backlog.cancel_old(client, args)
                self.assertFalse(backlog.api(client, backlog.CONSUMER_INFO).get("paused", False))
                self.assertEqual(deletes, [2])
                with contextlib.redirect_stdout(io.StringIO()):
                    backlog.cancel_old(client, args)
                for sequence in (2, 3, 7):
                    self.assertIsNone(backlog.api(client, backlog.MESSAGE_GET, {"seq": sequence}, missing_ok=True))
                for sequence in (1, 4, 5, 6, 8):
                    self.assertIsNotNone(backlog.api(client, backlog.MESSAGE_GET, {"seq": sequence}))
                final = backlog.api(client, backlog.CONSUMER_INFO)
                self.assertFalse(final.get("paused", False))
                self.assertEqual(final["num_ack_pending"], 1)
                self.assertEqual(final["num_pending"], 4)


if __name__ == "__main__":
    unittest.main()
