"""Bounded PDF extraction jobs. Only the worker container executes PDF tools."""
import concurrent.futures
import hmac
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tempfile
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_INPUT = 200 * 1024 * 1024
MAX_TEXT = 20 * 1024 * 1024
JOB_SECONDS = int(os.getenv("PDF_PARSER_JOB_TIMEOUT_SECONDS", "900"))
MAX_PAGES = int(os.getenv("PDF_PARSER_MAX_PAGES", "500"))
WORKERS = int(os.getenv("PDF_PARSER_WORKERS", "1"))
TOKEN = os.getenv("PDF_PARSER_TOKEN", "")


class ParseError(Exception):
    pass


class Job:
    def __init__(self, root):
        self.id = uuid.uuid4().hex
        self.root = Path(root)
        self.created = time.monotonic()
        self.cancel = threading.Event()
        self.lock = threading.RLock()
        self.state = "queued"
        self.events = []
        self.sequence = 0
        self.result = {}
        self.warnings = []
        self.error = ""
        self.discard = False
        self.future = None
        self.emit("pdf_waiting", "等待独立 PDF 解析服务调度")

    def emit(self, stage, message):
        with self.lock:
            self.sequence += 1
            self.events.append({"sequence": self.sequence, "stage": stage, "message": message})
            self.events = self.events[-128:]
        print(json.dumps({"job_id": self.id, "stage": stage, "message": message}, ensure_ascii=False), flush=True)

    def snapshot(self):
        with self.lock:
            return {"id": self.id, "state": self.state, "events": list(self.events),
                    "error": self.error, "result": dict(self.result)}

    def remaining(self):
        if self.cancel.is_set():
            raise ParseError("PDF 解析任务已取消")
        remaining = JOB_SECONDS - (time.monotonic() - self.created)
        if remaining <= 0:
            raise ParseError("PDF 解析超过任务总时限")
        return remaining


def run_tool(job, args, seconds=60, accepted=(0,)):
    """No shell. Kill the process group on cancellation/timeout; bound disk/output/RAM."""
    timeout = min(seconds, job.remaining())
    command = list(args)
    if shutil.which("prlimit"):
        command = ["prlimit", "--as=1073741824", "--fsize=67108864", "--nofile=256", "--"] + command
    started = time.monotonic()
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        proc = subprocess.Popen(command, stdout=out, stderr=err, start_new_session=True,
                                env={**os.environ, "LC_ALL": "C", "OMP_THREAD_LIMIT": "1"})
        try:
            while proc.poll() is None:
                job.remaining()
                if time.monotonic() - started > timeout:
                    raise ParseError(f"{args[0]} 超过 {int(timeout)} 秒时限")
                job.cancel.wait(0.1)
            out.seek(0)
            err.seek(0)
            output = out.read(64 * 1024).decode("utf-8", errors="replace")
            diagnostic = err.read(4096).decode("utf-8", errors="replace").strip()
            diagnostic = diagnostic.replace(str(job.root), "[临时文件]")
            if proc.returncode not in accepted:
                # Paths and tool diagnostics are bounded and never include the PDF body.
                raise ParseError(f"{args[0]} 失败（退出码 {proc.returncode}）：{diagnostic[:600]}")
            if diagnostic and len(job.warnings) < 20:
                job.warnings.append(f"{args[0]} 警告：{diagnostic[:600]}")
            return output, diagnostic
        finally:
            # Also remove descendants if the main process already exited.
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            proc.wait()


def page_count(job, source):
    output, _ = run_tool(job, ["pdfinfo", str(source)], 30)
    match = re.search(r"^Pages:\s+(\d+)", output, re.MULTILINE)
    if not match or int(match[1]) < 1:
        raise ParseError("PDF 未返回有效页数")
    count = int(match[1])
    if count > MAX_PAGES:
        raise ParseError(f"PDF 共 {count} 页，超过当前 {MAX_PAGES} 页上限")
    return count


def meaningful(text):
    # Page numbers/headers alone do not count as a usable text layer.
    text = text.strip()
    if sum(char.isalnum() for char in text) < 8:
        return False
    bad = sum(char == "\ufffd" or (ord(char) < 32 and char not in "\n\t\r") for char in text)
    return bad / max(1, len(text)) < 0.05


def extract_text(job, source, count):
    target = job.root / "extracted.txt"
    job.emit("pdf_text", f"使用 Poppler 提取现有文字，共 {count} 页")
    run_tool(job, ["pdftotext", "-enc", "UTF-8", "-layout", str(source), str(target)], 120)
    with target.open("rb") as stream:
        raw = stream.read(MAX_TEXT + 1)
    truncated = len(raw) > MAX_TEXT
    pages = raw[:MAX_TEXT].decode("utf-8", errors="replace").split("\f")
    return (pages + [""] * count)[:count], truncated


def repair(job, source):
    job.emit("pdf_repair", "使用 qpdf 修复 PDF 对象索引和结构，原文件保持不变")
    repaired = job.root / "repaired.pdf"
    _, warning = run_tool(job, ["qpdf", str(source), str(repaired)], 60, accepted=(0, 3))
    if not repaired.exists() or repaired.stat().st_size == 0:
        raise ParseError("qpdf 未生成可读取的修复副本")
    return repaired, warning


def parse_document(job):
    source = job.root / "input.pdf"
    warnings = job.warnings
    repaired = False
    try:
        count = page_count(job, source)
        pages, truncated = extract_text(job, source, count)
    except ParseError as initial_error:
        job.remaining()
        warnings.append(str(initial_error))
        source, warning = repair(job, source)
        repaired = True
        if warning:
            warnings.append("qpdf 已生成修复副本，但仍有结构警告")
        count = page_count(job, source)
        try:
            pages, truncated = extract_text(job, source, count)
        except ParseError as retry_error:
            job.remaining()
            warnings.append(str(retry_error))
            pages, truncated = [""] * count, False

    ocr_pages = 0
    missing = [i for i, text in enumerate(pages) if not meaningful(text)]
    if missing and not truncated:
        for index in missing:
            try:
                job.remaining()
                job.emit("pdf_ocr", f"OCR 中文/英文识别：第 {index + 1} / {count} 页，待识别 {len(missing)} 页")
                image = job.root / "page"
                run_tool(job, ["pdftoppm", "-f", str(index + 1), "-l", str(index + 1),
                               "-singlefile", "-scale-to", "2400", "-png", str(source), str(image)], 45)
                text_path = job.root / "ocr"
                ocr_started = time.monotonic()
                run_tool(job, ["tesseract", str(image) + ".png", str(text_path), "-l", "chi_sim+eng"], 60)
                text = text_path.with_suffix(".txt").read_text(encoding="utf-8")
                remaining = 60 - (time.monotonic() - ocr_started)
                if not meaningful(text) and remaining > 0:
                    job.emit("pdf_ocr", f"第 {index + 1} / {count} 页默认识别结果不足，尝试连续文本模式")
                    run_tool(job, ["tesseract", str(image) + ".png", str(text_path),
                                   "-l", "chi_sim+eng", "--psm", "6"], remaining)
                    text = text_path.with_suffix(".txt").read_text(encoding="utf-8")
                if meaningful(text):
                    pages[index] = text
                    ocr_pages += 1
                else:
                    warnings.append(f"第 {index + 1} 页未识别到有效正文（可能为空白页或纯图片）")
            except ParseError as error:
                warnings.append(f"第 {index + 1} 页：{error}")
                # Keep already extracted pages when the document's budget is exhausted.
                if job.cancel.is_set():
                    raise
                if time.monotonic() - job.created >= JOB_SECONDS:
                    break
            finally:
                for path in (job.root / "page.png", job.root / "ocr.txt"):
                    path.unlink(missing_ok=True)

    covered = sum(meaningful(text) for text in pages)
    if not covered:
        raise ParseError("未提取到可用正文；" + "；".join(warnings[-3:]))
    result_text = "\n\n".join(f"## 第 {i + 1} 页\n\n{text.strip()}" for i, text in enumerate(pages) if text.strip())
    raw = result_text.encode("utf-8")
    truncated = truncated or len(raw) > MAX_TEXT
    (job.root / "result.txt").write_bytes(raw[:MAX_TEXT].decode("utf-8", errors="ignore").encode("utf-8"))
    partial = truncated or covered < count
    message = f"正文覆盖 {covered} / {count} 页，其中 OCR 识别 {ocr_pages} 页"
    if truncated:
        message += "；正文超过 20 MB，已截断"
    if partial:
        message += "；部分页面未取得有效正文，请核对原文件"
    job.result = {"scope": "partial" if partial else "full", "parser": "Poppler + Tesseract" if ocr_pages else "Poppler",
                  "page_count": count, "extracted_pages": covered, "ocr_pages": ocr_pages,
                  "repaired": repaired, "truncated": truncated, "message": message, "warnings": warnings[:20]}
    job.emit("pdf_complete", message)


class Jobs:
    def __init__(self):
        self.lock = threading.RLock()
        self.items = {}
        self.slots = threading.BoundedSemaphore(WORKERS + 2)
        self.pool = concurrent.futures.ThreadPoolExecutor(max_workers=WORKERS)

    def cleanup(self):
        with self.lock:
            for key, job in list(self.items.items()):
                if job.state in ("completed", "failed") and time.monotonic() - job.created > JOB_SECONDS + 300:
                    self.items.pop(key)
                    shutil.rmtree(job.root, ignore_errors=True)

    def execute(self, job):
        terminal = "failed"
        try:
            job.state = "running"
            parse_document(job)
            terminal = "completed"
        except Exception as error:
            job.error = str(error)[:2000]
            job.emit("pdf_failed", job.error)
        finally:
            try:
                with self.lock:
                    for path in job.root.iterdir():
                        if path.name != "result.txt":
                            path.unlink(missing_ok=True)
                    job.state = terminal
                    if job.discard:
                        shutil.rmtree(job.root, ignore_errors=True)
            finally:
                self.slots.release()


JOBS = Jobs()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def json_response(self, status, body):
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def authorized(self):
        self.connection.settimeout(90)
        JOBS.cleanup()
        if TOKEN and not hmac.compare_digest(self.headers.get("Authorization", "").encode("utf-8"), ("Bearer " + TOKEN).encode("utf-8")):
            self.json_response(401, {"error": "unauthorized"})
            return False
        return True

    def do_POST(self):
        if not self.authorized():
            return
        if self.path != "/jobs":
            self.json_response(404, {"error": "not found"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = 0
        if not 0 < length <= MAX_INPUT:
            self.json_response(413, {"error": "PDF 大小必须在 1 字节至 200 MB 之间"})
            return
        if not JOBS.slots.acquire(blocking=False):
            self.json_response(429, {"error": "PDF 解析服务繁忙，等待其他文档完成后重试"})
            return
        root = tempfile.mkdtemp(prefix="pandawiki-pdf-")
        try:
            with open(Path(root) / "input.pdf", "wb") as target:
                remaining = length
                while remaining:
                    block = self.rfile.read(min(remaining, 1024 * 1024))
                    if not block:
                        raise ParseError("PDF 上传中断")
                    target.write(block)
                    remaining -= len(block)
            job = Job(root)
            with JOBS.lock:
                # Bound retained results even if a client disappears before DELETE.
                finished = [j for j in JOBS.items.values() if j.state in ("completed", "failed")]
                for old in sorted(finished, key=lambda j: j.created)[:-15]:
                    JOBS.items.pop(old.id)
                    shutil.rmtree(old.root, ignore_errors=True)
                JOBS.items[job.id] = job
            job.future = JOBS.pool.submit(JOBS.execute, job)
        except Exception as error:
            JOBS.slots.release()
            shutil.rmtree(root, ignore_errors=True)
            self.json_response(400, {"error": str(error)[:600]})
            return
        self.json_response(202, {"id": job.id})

    def do_GET(self):
        if self.path == "/health":
            ready = all(shutil.which(tool) for tool in ("pdfinfo", "pdftotext", "qpdf", "pdftoppm", "tesseract", "prlimit"))
            self.json_response(200 if ready else 503, {"ready": ready})
            return
        if not self.authorized():
            return
        parts = self.path.strip("/").split("/")
        with JOBS.lock:
            job = JOBS.items.get(parts[1]) if len(parts) >= 2 and parts[0] == "jobs" else None
            if job is None:
                self.json_response(404, {"error": "PDF 解析任务不存在或已过期，请重新解析"})
            elif len(parts) == 2:
                self.json_response(200, job.snapshot())
            elif len(parts) == 3 and parts[2] == "text" and job.state == "completed":
                target = job.root / "result.txt"
                self.send_response(200)
                self.send_header("Content-Type", "text/plain; charset=utf-8")
                self.send_header("Content-Length", str(target.stat().st_size))
                self.end_headers()
                with target.open("rb") as stream:
                    shutil.copyfileobj(stream, self.wfile)
            else:
                self.json_response(409, {"error": "PDF 正文尚未生成"})

    def do_DELETE(self):
        if not self.authorized():
            return
        parts = self.path.strip("/").split("/")
        with JOBS.lock:
            job = JOBS.items.pop(parts[1], None) if len(parts) == 2 and parts[0] == "jobs" else None
            if job:
                job.discard = True
                job.cancel.set()
                cancelled_before_start = job.future is not None and job.future.cancel()
                if cancelled_before_start:
                    JOBS.slots.release()
                if cancelled_before_start or job.state in ("completed", "failed"):
                    shutil.rmtree(job.root, ignore_errors=True)
        self.json_response(200, {"deleted": bool(job)})


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8090), Handler).serve_forever()
