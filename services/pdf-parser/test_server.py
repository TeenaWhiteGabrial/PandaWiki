import os
import concurrent.futures
import ctypes
import ctypes.util
import http.client
from pathlib import Path
import subprocess
import tempfile
import threading
import unittest
import zlib
from unittest.mock import patch

import server


@unittest.skipUnless(os.getenv("PDF_PARSER_REAL_TOOLS") == "1", "requires local HTTP sockets")
class HTTPBoundaryTests(unittest.TestCase):
    def setUp(self):
        token = patch.object(server, "TOKEN", "fixture-token")
        token.start()
        self.addCleanup(token.stop)
        self.http = server.ThreadingHTTPServer(("127.0.0.1", 0), server.Handler)
        thread = threading.Thread(target=self.http.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(self.http.server_close)
        self.addCleanup(self.http.shutdown)

    def request(self, method, path, headers):
        conn = http.client.HTTPConnection("127.0.0.1", self.http.server_port, timeout=3)
        self.addCleanup(conn.close)
        conn.request(method, path, headers=headers)
        response = conn.getresponse()
        response.read()
        return response.status

    def test_token_and_malformed_token_are_rejected(self):
        self.assertEqual(self.request("GET", "/jobs/missing", {}), 401)
        self.assertEqual(self.request("GET", "/jobs/missing", {"Authorization": b"Bearer \xff"}), 401)
        self.assertEqual(self.request("GET", "/jobs/missing", {"Authorization": "Bearer fixture-token"}), 404)

    def test_oversized_upload_is_rejected_before_reading(self):
        self.assertEqual(self.request("POST", "/jobs", {"Authorization": "Bearer fixture-token", "Content-Length": str(server.MAX_INPUT + 1)}), 413)

    def test_cancelled_queued_job_releases_upload_slot(self):
        jobs = server.Jobs()
        self.addCleanup(jobs.pool.shutdown)
        root = tempfile.mkdtemp()
        self.addCleanup(lambda: server.shutil.rmtree(root, ignore_errors=True))
        job = server.Job(root)
        job.future = concurrent.futures.Future()
        jobs.items[job.id] = job
        self.assertTrue(jobs.slots.acquire(blocking=False))
        with patch.object(server, "JOBS", jobs):
            self.assertEqual(self.request("DELETE", "/jobs/" + job.id, {"Authorization": "Bearer fixture-token"}), 200)
        self.assertTrue(job.future.cancelled())
        self.assertFalse(Path(root).exists())
        for _ in range(server.WORKERS + 2):
            self.assertTrue(jobs.slots.acquire(blocking=False))


def write_pdf(path, objects, broken=False):
    data = bytearray(b"%PDF-1.4\n")
    offsets = []
    for index, value in enumerate(objects, 1):
        offsets.append(len(data))
        data.extend(f"{index} 0 obj\n".encode() + value + b"\nendobj\n")
    if broken:
        offsets[1] = offsets[2]
    start = len(data)
    data.extend(f"xref\n0 {len(objects) + 1}\n0000000000 65535 f \n".encode())
    for offset in offsets:
        data.extend(f"{offset:010d} 00000 n \n".encode())
    data.extend(f"trailer\n<< /Size {len(objects) + 1} /Root 1 0 R >>\nstartxref\n{start}\n%%EOF\n".encode())
    path.write_bytes(data)


def text_pdf(path, broken=False):
    content = b"BT /F1 32 Tf 72 650 Td (SAMPLE DOCUMENT TEXT) Tj ET\n"
    write_pdf(path, [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
        f"<< /Length {len(content)} >>\nstream\n".encode() + content + b"endstream",
        b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    ], broken)


class ParserTests(unittest.TestCase):
    def new_job(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        job = server.Job(directory.name)
        (job.root / "input.pdf").write_bytes(b"%PDF-1.4\n")
        return job

    def test_repair_then_extract_accepts_warning_exit(self):
        job = self.new_job()
        attempts = 0

        def tool(job, args, seconds=60, accepted=(0,)):
            nonlocal attempts
            if args[0] == "pdfinfo":
                return "Pages: 1\n", ""
            if args[0] == "pdftotext":
                attempts += 1
                if attempts == 1:
                    raise server.ParseError("object reference mismatch")
                Path(args[-1]).write_text("Recovered original text\f", encoding="utf-8")
            if args[0] == "qpdf":
                self.assertIn(3, accepted)
                Path(args[-1]).write_bytes(b"%PDF-repaired")
                return "", "reconstructed xref"
            return "", ""

        with patch.object(server, "run_tool", side_effect=tool):
            server.parse_document(job)
        self.assertEqual(job.result["scope"], "full")
        self.assertTrue(job.result["repaired"])
        self.assertIn("pdf_repair", [event["stage"] for event in job.events])

    def test_mixed_document_keeps_text_and_ocr_pages(self):
        job = self.new_job()
        rendered = []

        def tool(job, args, seconds=60, accepted=(0,)):
            if args[0] == "pdfinfo":
                return "Pages: 3\n", ""
            if args[0] == "pdftotext":
                Path(args[-1]).write_text("Original searchable text\f\f\f", encoding="utf-8")
            if args[0] == "pdftoppm":
                rendered.append(args[args.index("-f") + 1])
                Path(args[-1] + ".png").write_bytes(b"image")
            if args[0] == "tesseract":
                Path(args[2] + ".txt").write_text("扫描文档中文识别成功" if rendered[-1] == "2" else "", encoding="utf-8")
            return "", ""

        with patch.object(server, "run_tool", side_effect=tool):
            server.parse_document(job)
        self.assertEqual(rendered, ["2", "3"])
        self.assertEqual(job.result["scope"], "partial")
        self.assertEqual(job.result["extracted_pages"], 2)
        text = (job.root / "result.txt").read_text(encoding="utf-8")
        self.assertIn("Original searchable text", text)
        self.assertIn("扫描文档中文识别成功", text)

    def test_all_unreadable_is_failure(self):
        job = self.new_job()
        with patch.object(server, "run_tool", side_effect=server.ParseError("unreadable")):
            with self.assertRaises(server.ParseError):
                server.parse_document(job)

    def test_command_timeout_and_cancellation(self):
        job = self.new_job()
        with self.assertRaisesRegex(server.ParseError, "时限"):
            server.run_tool(job, ["python3", "-c", "import time; time.sleep(30)"], seconds=0.1)
        job.cancel.set()
        with self.assertRaisesRegex(server.ParseError, "取消"):
            server.run_tool(job, ["python3", "-c", "print('must not run')"])


@unittest.skipUnless(os.getenv("PDF_PARSER_REAL_TOOLS") == "1", "requires container PDF/OCR tools")
class RealToolTests(unittest.TestCase):
    def new_job(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        return server.Job(directory.name)

    def test_normal_and_wrong_xref_documents(self):
        for broken in (False, True):
            with self.subTest(broken=broken):
                job = self.new_job()
                text_pdf(job.root / "input.pdf", broken)
                server.parse_document(job)
                self.assertEqual(job.result["scope"], "full")
                self.assertIn("SAMPLE DOCUMENT TEXT", (job.root / "result.txt").read_text())

    def test_qpdf_recovers_wrong_xref(self):
        job = self.new_job()
        source = job.root / "input.pdf"
        text_pdf(source, True)
        repaired, _ = server.repair(job, source)
        self.assertEqual(server.page_count(job, repaired), 1)

    def scan_pdf(self, job, source):
        server.run_tool(job, ["pdftoppm", "-singlefile", "-r", "150", str(source), str(job.root / "scan")])
        magic, dimensions, maximum, pixels = (job.root / "scan.ppm").read_bytes().split(b"\n", 3)
        self.assertEqual((magic, maximum), (b"P6", b"255"))
        width, height = map(int, dimensions.split())
        self.image_pdf(job, width, height, pixels)

    def image_pdf(self, job, width, height, pixels):
        image = zlib.compress(pixels)
        content = b"q 500 0 0 100 56 500 cm /Im1 Do Q\n" if height < width else b"q 612 0 0 792 0 0 cm /Im1 Do Q\n"
        write_pdf(job.root / "input.pdf", [
            b"<< /Type /Catalog /Pages 2 0 R >>",
            b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
            b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>",
            f"<< /Length {len(content)} >>\nstream\n".encode() + content + b"endstream",
            f"<< /Type /XObject /Subtype /Image /Width {width} /Height {height} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length {len(image)} >>\nstream\n".encode() + image + b"\nendstream",
        ])

    def test_image_only_document_ocr(self):
        job = self.new_job()
        source = job.root / "original.pdf"
        text_pdf(source)
        self.scan_pdf(job, source)
        server.parse_document(job)
        self.assertEqual(job.result["scope"], "full")
        self.assertEqual(job.result["ocr_pages"], 1)
        self.assertIn("SAMPLE", (job.root / "result.txt").read_text())

    def chinese_scan(self, job, text):
        # Draw an actual Chinese scan with a real Unicode font. The minimal
        # text-layer fixture above deliberately omits embedded glyph programs.
        cairo = ctypes.CDLL(ctypes.util.find_library("cairo"))
        pointer, integer, number = ctypes.c_void_p, ctypes.c_int, ctypes.c_double
        signatures = {
            "cairo_image_surface_create": (pointer, [integer, integer, integer]),
            "cairo_create": (pointer, [pointer]),
            "cairo_set_source_rgb": (None, [pointer, number, number, number]),
            "cairo_paint": (None, [pointer]),
            "cairo_select_font_face": (None, [pointer, ctypes.c_char_p, integer, integer]),
            "cairo_set_font_size": (None, [pointer, number]),
            "cairo_move_to": (None, [pointer, number, number]),
            "cairo_show_text": (None, [pointer, ctypes.c_char_p]),
            "cairo_surface_flush": (None, [pointer]),
            "cairo_image_surface_get_stride": (integer, [pointer]),
            "cairo_image_surface_get_data": (pointer, [pointer]),
            "cairo_destroy": (None, [pointer]),
            "cairo_surface_destroy": (None, [pointer]),
        }
        for name, (result_type, args) in signatures.items():
            getattr(cairo, name).restype = result_type
            getattr(cairo, name).argtypes = args
        width, height = 1200, 240
        surface = cairo.cairo_image_surface_create(0, width, height)
        context = cairo.cairo_create(surface)
        try:
            cairo.cairo_set_source_rgb(context, 1, 1, 1)
            cairo.cairo_paint(context)
            cairo.cairo_set_source_rgb(context, 0, 0, 0)
            cairo.cairo_select_font_face(context, b"Noto Sans CJK SC", 0, 0)
            cairo.cairo_set_font_size(context, 70)
            cairo.cairo_move_to(context, 50, 140)
            cairo.cairo_show_text(context, text.encode("utf-8"))
            cairo.cairo_surface_flush(surface)
            stride = cairo.cairo_image_surface_get_stride(surface)
            data = ctypes.string_at(cairo.cairo_image_surface_get_data(surface), stride * height)
            pixels = bytearray()
            for row in range(height):
                for col in range(width):
                    offset = row * stride + col * 4
                    pixels.extend((data[offset + 2], data[offset + 1], data[offset]))
            self.image_pdf(job, width, height, pixels)
        finally:
            cairo.cairo_destroy(context)
            cairo.cairo_surface_destroy(surface)

    def test_chinese_text_and_scanned_document(self):
        job = self.new_job()
        source = job.root / "original.pdf"
        text = "企业安全生产管理制度文件"
        content = f"BT /F1 32 Tf 72 650 Td <{text.encode('utf-16-be').hex()}> Tj ET\n".encode()
        write_pdf(source, [
            b"<< /Type /Catalog /Pages 2 0 R >>",
            b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
            b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
            f"<< /Length {len(content)} >>\nstream\n".encode() + content + b"endstream",
            b"<< /Type /Font /Subtype /Type0 /BaseFont /STSong-Light /Encoding /UniGB-UCS2-H /DescendantFonts [6 0 R] >>",
            b"<< /Type /Font /Subtype /CIDFontType0 /BaseFont /STSong-Light /CIDSystemInfo << /Registry (Adobe) /Ordering (GB1) /Supplement 4 >> /DW 1000 >>",
        ])
        count = server.page_count(job, source)
        pages, _ = server.extract_text(job, source, count)
        self.assertIn("安全生产", "".join(pages[0].split()))
        self.chinese_scan(job, text)
        server.parse_document(job)
        self.assertEqual(job.result["ocr_pages"], 1)
        self.assertIn("安全生产", "".join((job.root / "result.txt").read_text().split()))

    def test_chinese_and_english_language_packs(self):
        languages = subprocess.check_output(["tesseract", "--list-langs"], text=True)
        self.assertIn("chi_sim", languages)
        self.assertIn("eng", languages)


if __name__ == "__main__":
    unittest.main()
