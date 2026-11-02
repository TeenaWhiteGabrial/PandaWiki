#!/usr/bin/env python3
"""RAGLite queue inspection; Python standard library, no pip install.

Default mode only reads information and a few stored messages. Optional
--cancel-old previews scoped legacy jobs; only --apply pauses delivery and removes
matching unclaimed messages after backup. Never ACKs, purges, or changes documents.
Credentials are read from Docker configuration and never printed.
"""

import argparse
import base64
from collections import Counter
from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import sys
import time
import urllib.request
import uuid


def inspect_container(name):
    result = subprocess.run(
        ["docker", "inspect", name], capture_output=True, text=True, check=False
    )
    if result.returncode:
        raise RuntimeError("无法读取容器配置：" + name)
    return json.loads(result.stdout)[0]


class NatsReader:
    READ_APIS = (
        "$JS.API.STREAM.INFO.raglite_tasks",
        "$JS.API.CONSUMER.INFO.raglite_tasks.raglite_workers",
        "$JS.API.STREAM.MSG.GET.raglite_tasks",
    )

    def __init__(self, host, port, username, password):
        self.sock = socket.create_connection((host, port), timeout=15)
        self.reader = self.sock.makefile("rb")
        if not self.reader.readline().startswith(b"INFO "):
            raise RuntimeError("未收到 NATS 服务信息")
        self.inbox = "_INBOX.pandawiki_inspect_" + uuid.uuid4().hex
        connect = json.dumps({
            "user": username, "pass": password, "verbose": False,
            "pedantic": False, "lang": "python", "version": "1",
            "name": "pandawiki-readonly-backlog-inspector",
        }).encode()
        self.sock.sendall(b"CONNECT " + connect + b"\r\nSUB " +
                          self.inbox.encode() + b" 1\r\n")

    def request(self, subject, payload=None):
        # Keep this helper restricted to the three read-only API operations.
        if subject not in self.READ_APIS:
            raise RuntimeError("拒绝非只读请求")
        return self._request(subject, payload)

    def _request(self, subject, payload=None):
        data = json.dumps(payload or {}).encode()
        command = "PUB {} {} {}\r\n".format(subject, self.inbox, len(data))
        self.sock.sendall(command.encode() + data + b"\r\n")
        while True:
            line = self.reader.readline()
            if not line:
                raise RuntimeError("NATS 连接关闭")
            if line == b"PING\r\n":
                self.sock.sendall(b"PONG\r\n")
            elif line.startswith(b"-ERR"):
                raise RuntimeError("NATS 拒绝请求；请核对容器内的认证配置")
            elif line.startswith(b"MSG "):
                size = int(line.split()[-1])
                if size > 32 * 1024 * 1024:
                    raise RuntimeError("消息超过检查工具的大小限制")
                data = self.reader.read(size)
                if self.reader.read(2) != b"\r\n":
                    raise RuntimeError("NATS 响应不完整")
                return json.loads(data)

    def close(self):
        self.reader.close()
        self.sock.close()


def describe_payload(data):
    """Only expose identifier fields and schema keys, never document content."""
    try:
        value = json.loads(base64.b64decode(data, validate=True))
    except (ValueError, TypeError):
        return {"格式": "非 JSON；未输出正文"}
    result = {"字段结构": [], "标识字段": {}}
    allowed = {"datasetid", "documentid", "docid", "taskid", "tasktype", "type", "action"}

    def visit(item, path, depth):
        if depth > 5 or not isinstance(item, dict):
            return
        result["字段结构"].append({"路径": path, "字段": list(item)[:40]})
        for key, child in item.items():
            label = path + "." + key
            normalized = key.replace("_", "").replace("-", "").lower()
            if normalized in allowed and isinstance(child, (str, int)):
                result["标识字段"][label] = str(child)[:128]
            if isinstance(child, str) and child.lstrip().startswith("{"):
                try:
                    child = json.loads(child)
                except ValueError:
                    pass
            visit(child, label, depth + 1)

    visit(value, "$", 0)
    return result


def connect_nats(name):
    container = inspect_container(name)
    command = container["Config"].get("Cmd") or []

    def flag(name):
        for index, item in enumerate(command):
            if item == name and index + 1 < len(command):
                return command[index + 1]
            if item.startswith(name + "="):
                return item.split("=", 1)[1]
        raise RuntimeError("未找到 " + name + "；工具不会猜测认证配置")

    bindings = container["NetworkSettings"]["Ports"].get("4222/tcp") or []
    if not bindings:
        raise RuntimeError("NATS 4222 没有宿主机映射；请保留原配置并反馈此提示")
    binding = next((b for b in bindings if ":" not in b["HostIp"]), bindings[0])
    host = binding["HostIp"]
    if host in ("", "0.0.0.0", "::"):
        host = "127.0.0.1" if host != "::" else "::1"
    return NatsReader(host, int(binding["HostPort"]), flag("--user"), flag("--pass"))


STREAM_INFO = "$JS.API.STREAM.INFO.raglite_tasks"
CONSUMER_INFO = "$JS.API.CONSUMER.INFO.raglite_tasks.raglite_workers"
MESSAGE_GET = "$JS.API.STREAM.MSG.GET.raglite_tasks"
MESSAGE_DELETE = "$JS.API.STREAM.MSG.DELETE.raglite_tasks"
CONSUMER_PAUSE = "$JS.API.CONSUMER.PAUSE.raglite_tasks.raglite_workers"


def api(client, subject, payload=None, missing_ok=False, mutate=False):
    if mutate:
        if subject not in (MESSAGE_DELETE, CONSUMER_PAUSE):
            raise RuntimeError("拒绝范围外的写入请求")
        result = client._request(subject, payload)
    else:
        result = client.request(subject, payload)
    if "error" in result:
        error = result["error"]
        if missing_ok and error.get("err_code") == 10037:  # JSNoMessageFoundErr
            return None
        raise RuntimeError("NATS API 请求失败，code={} err_code={}".format(
            error.get("code"), error.get("err_code")))
    return result


def parse_time(value):
    # Python 3.10 accepts microseconds, whereas NATS publishes nanoseconds.
    normalized = re.sub(r"(\.\d{6})\d+", r"\1", value).replace("Z", "+00:00")
    parsed = datetime.fromisoformat(normalized)
    if parsed.tzinfo is None:
        raise RuntimeError("时间必须包含时区，例如 2026-10-30T01:18:03Z")
    return parsed.astimezone(timezone.utc)


def target_message(message, delivered, args):
    if not delivered < message["seq"] <= args.through_seq:
        return False
    if message["subject"] != "raglite.tasks.document_indexing":
        return False
    if parse_time(message["time"]) > parse_time(args.published_before):
        return False
    try:
        payload = json.loads(base64.b64decode(message["data"], validate=True))
    except (ValueError, TypeError):
        return False
    return (isinstance(payload, dict)
            and payload.get("type") == "document_indexing"
            and payload.get("dataset_id") == args.dataset_id
            and payload.get("task_id") == ""
            and isinstance(payload.get("document_id"), str)
            and bool(payload["document_id"]))


def read_queue(client, stream, consumer, args):
    state = stream["state"]
    first, last = state["first_seq"], state["last_seq"]
    if last - first > 100000:
        raise RuntimeError("序号跨度超过 10 万；未删除任何消息，请先检查队列")
    records = []
    for sequence in range(first, last + 1):
        reply = api(client, MESSAGE_GET, {"seq": sequence}, missing_ok=True)
        if reply is not None:
            message = reply["message"]
            if message["seq"] != sequence:
                raise RuntimeError("消息序号不一致")
            records.append({"remove": target_message(
                message, consumer["delivered"]["stream_seq"], args), "message": message})
    return records


def document_states(args):
    """Optional HTTP GET only; retain statuses, not titles, bodies or credentials."""
    container = inspect_container(args.raglite_container)
    env = dict(item.split("=", 1) for item in container["Config"].get("Env", []) if "=" in item)
    bindings = container["NetworkSettings"]["Ports"].get("5050/tcp") or []
    if not bindings or not env.get("API_KEY"):
        raise RuntimeError("未找到 RAGLite 本地端口或 API_KEY")
    binding = next((b for b in bindings if ":" not in b["HostIp"]), bindings[0])
    host = binding["HostIp"]
    if host in ("", "0.0.0.0", "::"):
        host = "127.0.0.1" if host != "::" else "::1"
    if ":" in host:
        host = "[" + host + "]"
    url = "http://{}:{}/api/v1/datasets/{}/documents?page_size=0".format(
        host, int(binding["HostPort"]), args.dataset_id)
    request = urllib.request.Request(url, headers={"Authorization": "Bearer " + env["API_KEY"]})
    # Never send credentials through a configured outbound HTTP proxy or redirect.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *unused):
            return None
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    with opener.open(request, timeout=30) as response:
        body = json.load(response)
    if body.get("success") is not True:
        raise RuntimeError("RAGLite 文档状态查询未成功")
    documents = body["data"]["documents"]
    if len(documents) != body["data"]["total"]:
        raise RuntimeError("文档列表不完整")
    return {doc["id"]: {key: doc.get(key) for key in ("status", "updated_at")}
            for doc in documents}


def durable_json(path, data):
    with path.open("x", encoding="utf-8") as handle:
        os.chmod(path, 0o600)
        json.dump(data, handle, ensure_ascii=False)
        handle.write("\n")
        handle.flush()
        os.fsync(handle.fileno())


def cancel_old(client, args):
    # Applying requires explicit scope. No stream PURGE, document deletion or DB writes.
    uuid.UUID(args.dataset_id)
    if args.through_seq <= 0:
        raise RuntimeError("through-seq 必须大于 0")
    parse_time(args.published_before)
    stream = api(client, STREAM_INFO)
    consumer = api(client, CONSUMER_INFO)
    if stream["config"].get("retention") != "workqueue" or stream["state"]["consumer_count"] != 1:
        raise RuntimeError("需要 workqueue 且仅有 raglite_workers 一个消费者；未修改队列")
    if consumer.get("paused"):
        raise RuntimeError("消费者已经暂停；为保留原来的运维状态，工具未作修改")
    if consumer["config"].get("deliver_policy") != "all":
        raise RuntimeError("消费者不是从头投递模式，无法可靠保护已领取任务")
    owned_pause = False
    started = time.monotonic()
    pause_until = (datetime.now(timezone.utc) + timedelta(minutes=10)).isoformat().replace("+00:00", "Z")
    directory = None

    def check_pause():
        if time.monotonic() - started > 480:
            raise RuntimeError("操作超过 8 分钟，已停止进一步删除")
        current = api(client, CONSUMER_INFO)
        if (not current.get("paused")
                or current["delivered"]["stream_seq"] != consumer["delivered"]["stream_seq"]
                or current.get("created") != consumer.get("created")
                or parse_time(current["config"]["pause_until"]) != parse_time(pause_until)):
            raise RuntimeError("暂停状态或消费者位置变化，已停止进一步删除")
        if api(client, STREAM_INFO).get("created") != stream.get("created"):
            raise RuntimeError("流被重建，已停止进一步删除")

    try:
        if args.apply:
            owned_pause = True  # Also attempt recovery if the response is lost.
            reply = api(client, CONSUMER_PAUSE, {"pause_until": pause_until}, mutate=True)
            if not reply.get("paused"):
                raise RuntimeError("暂停领取未成功，未删除消息")
            stream = api(client, STREAM_INFO)
            consumer = api(client, CONSUMER_INFO)
            check_pause()
            print("已短暂暂停领取新任务；已领取任务可继续执行。", flush=True)
        records = read_queue(client, stream, consumer, args)
        targets = [record["message"] for record in records if record["remove"]]
        doc_ids = {json.loads(base64.b64decode(m["data"]))["document_id"] for m in targets}
        statuses = {}
        try:
            statuses = document_states(args)
        except (RuntimeError, OSError, ValueError, KeyError):
            print("提示：未能读取文档状态；本工具仍仅处理队列，不修改文档状态。", flush=True)
        print(json.dumps({
            "模式": "执行取消" if args.apply else "只读预览；未暂停或删除任务",
            "数据集": args.dataset_id, "最大旧序号": args.through_seq,
            "入队时间上限": args.published_before,
            "匹配的未领取旧任务": len(targets), "对应不同文档数": len(doc_ids),
            "已领取未确认_保留": consumer["num_ack_pending"],
            "其余保留消息数": len(records) - len(targets),
            "对应文档当前状态": dict(Counter(statuses.get(i, {}).get("status", "未知") for i in doc_ids)),
        }, ensure_ascii=False, indent=2), flush=True)
        if not args.apply or not targets:
            return
        check_pause()
        root = Path(args.backup_dir)
        root.mkdir(mode=0o700, parents=True, exist_ok=True)
        directory = root / ("cancel-" + datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + uuid.uuid4().hex[:8])
        directory.mkdir(mode=0o700)
        durable_json(directory / "snapshot.json", {
            "stream": stream, "consumer": consumer, "dataset_id": args.dataset_id,
            "through_seq": args.through_seq, "published_before": args.published_before,
            "documents": statuses, "note": "队列取消备份，不是完整数据库/向量备份；未修改文档状态",
        })
        with (directory / "messages.jsonl").open("x", encoding="utf-8") as backup:
            os.chmod(directory / "messages.jsonl", 0o600)
            for record in records:
                backup.write(json.dumps(record, ensure_ascii=False) + "\n")
            backup.flush()
            os.fsync(backup.fileno())
        descriptor = os.open(str(directory), os.O_RDONLY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)
        print("已保存取消前的消息备份：" + str(directory), flush=True)
        removed = 0
        with (directory / "removed.jsonl").open("x", encoding="utf-8") as receipt:
            os.chmod(directory / "removed.jsonl", 0o600)
            for message in targets:
                check_pause()
                # Re-read identity, including time/data, in case the stream changed.
                current = api(client, MESSAGE_GET, {"seq": message["seq"]}, missing_ok=True)
                if current is None:
                    continue
                if current["message"] != message:
                    raise RuntimeError("消息内容变化，已停止进一步删除")
                reply = api(client, MESSAGE_DELETE, {"seq": message["seq"], "no_erase": False}, mutate=True)
                if not reply.get("success"):
                    raise RuntimeError("消息删除未确认；请保留备份和输出")
                removed += 1
                receipt.write(json.dumps({"sequence": message["seq"]}) + "\n")
                receipt.flush()
                if removed % 100 == 0:
                    os.fsync(receipt.fileno())
                    print("已取消旧队列任务：{} / {}".format(removed, len(targets)), flush=True)
            os.fsync(receipt.fileno())
        # Verify exact selected sequences, rather than only an aggregate count.
        for message in targets:
            if api(client, MESSAGE_GET, {"seq": message["seq"]}, missing_ok=True) is not None:
                raise RuntimeError("仍存在目标旧消息，请保留备份和输出")
        final = api(client, CONSUMER_INFO)
        durable_json(directory / "result.json", {"removed": removed, "consumer": final})
        print(json.dumps({"实际取消消息": removed, "目标消息已逐条确认移除": True,
                          "当前等待领取": final["num_pending"],
                          "当前已领取未确认_保留": final["num_ack_pending"],
                          "文档状态": "未修改；PENDING/RUNNING 状态需单独核对收尾"},
                         ensure_ascii=False, indent=2), flush=True)
    finally:
        if owned_pause:
            try:
                # Fresh connection also handles a broken original connection.
                recovery = connect_nats(args.nats_container)
                try:
                    current = api(recovery, CONSUMER_INFO)
                    if current.get("paused"):
                        if parse_time(current["config"]["pause_until"]) != parse_time(pause_until):
                            raise RuntimeError("消费者已被其他操作暂停；保留其状态")
                        reply = api(recovery, CONSUMER_PAUSE, {}, mutate=True)
                        if reply.get("paused"):
                            raise RuntimeError("恢复领取未确认")
                finally:
                    recovery.close()
                print("已恢复正常领取；新任务可继续处理。", flush=True)
            except (RuntimeError, OSError, ValueError, KeyError):
                print("警告：未确认恢复领取。本工具设置的暂停到 {} 到期；请核对是否有其他暂停操作。".format(pause_until),
                      file=sys.stderr, flush=True)
                raise RuntimeError("取消流程未完整结束，需要核对消费者暂停状态")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--nats-container", default="pandawiki-nats")
    parser.add_argument("--raglite-container", default="pandawiki-raglite")
    parser.add_argument("--cancel-old", action="store_true", help="预览取消指定范围的旧任务")
    parser.add_argument("--dataset-id")
    parser.add_argument("--through-seq", type=int)
    parser.add_argument("--published-before", help="旧消息入队时间上限，ISO 8601")
    parser.add_argument("--apply", action="store_true", help="显式执行取消，先备份后删除")
    parser.add_argument("--backup-dir", default="/root/pandawiki-raglite-cancel-backups")
    args = parser.parse_args()
    if args.apply and not args.cancel_old:
        parser.error("--apply 只能与 --cancel-old 一起使用")
    if args.cancel_old and not (args.dataset_id and args.through_seq and args.published_before):
        parser.error("取消预览/执行必须指定 dataset-id、through-seq、published-before")
    client = connect_nats(args.nats_container)
    try:
        if args.cancel_old:
            cancel_old(client, args)
            return
        stream = client.request("$JS.API.STREAM.INFO.raglite_tasks")
        consumer = client.request("$JS.API.CONSUMER.INFO.raglite_tasks.raglite_workers")
        if "error" in stream or "error" in consumer:
            raise RuntimeError("无法读取 raglite_tasks / raglite_workers，请检查名称和权限")
        state = stream["state"]
        delivered = consumer["delivered"]["stream_seq"]
        print(json.dumps({
            "说明": "只读快照；未消费、确认或删除任何消息；运行中数字可能变化",
            "流中消息": state["messages"], "等待领取": consumer["num_pending"],
            "已领取未确认": consumer["num_ack_pending"],
            "最早消息时间": state.get("first_ts"), "最后消息时间": state.get("last_ts"),
            "最后已投递序号": delivered, "当前流末尾序号": state["last_seq"],
        }, ensure_ascii=False, indent=2), flush=True)
        # Direct GET never delivers work to a worker or changes its ACK state.
        candidates = [max(state["first_seq"], delivered + 1),
                      (delivered + state["last_seq"]) // 2, state["last_seq"]]
        seen = set()
        for sequence in candidates:
            if sequence <= delivered or sequence > state["last_seq"] or sequence in seen:
                continue
            seen.add(sequence)
            reply = client.request("$JS.API.STREAM.MSG.GET.raglite_tasks", {"seq": sequence})
            message = reply.get("message")
            if not message:
                print("样本序号 {}：当前不存在，可能已被处理".format(sequence), flush=True)
                continue
            report = {"序号": message["seq"], "主题": message["subject"],
                      "入队时间": message["time"],
                      "任务结构": describe_payload(message.get("data", ""))}
            print(json.dumps(report, ensure_ascii=False, indent=2), flush=True)
    finally:
        client.close()


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, KeyError, IndexError) as error:
        print("检查失败：" + str(error), file=sys.stderr)
        sys.exit(1)
