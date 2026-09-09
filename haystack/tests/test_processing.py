"""用真实格式验证正文提取、字符覆盖和分段位置。"""

import json
import tempfile
import unittest
from pathlib import Path

from docx import Document as WordDocument
from fastapi import HTTPException
from openpyxl import Workbook
from pptx import Presentation
from pptx.util import Inches
from pypdf import PdfWriter

from knowledge import convert_file, split_documents
from haystack import Document


class ProcessingTests(unittest.TestCase):
    """检查不同文档格式与中文分段边界。"""

    def test_character_coverage(self):
        """去除重叠后完整恢复原文，含中文、金额、代码及 emoji。"""
        for length, overlap in [(256, 0), (256, 30), (256, 200), (512, 200), (2048, 50)]:
            for text in [("合同金额 1,234.50 元，不含税。\n    code(x)\n\n🙂备注：不得退款！" * 120), "连续中文" * 3000, "\n\n".join(f"第 {i} 节：唯一标题\n" + (f"本节序号 {i}。正文保留完整。" * 30) for i in range(10))]:
                segments = split_documents([Document(content=text)], length, overlap)
                restored = ""
                for segment in segments:
                    content = segment["content"]
                    self.assertLessEqual(len(content), length)
                    self.assertEqual(segment["character_count"], len(content))
                    # 按原文偏移核对分段内容和字符覆盖范围。
                    start = max(0, len(restored) - overlap)
                    self.assertTrue(text.startswith(content, start), (length, overlap, start))
                    restored = text[:start] + content
                self.assertEqual(restored, text)

    def test_supported_formats(self):
        """验证各类文件的正文、工作表名称和 PDF 页码。"""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for suffix, text in {
                ".txt": "合同金额 1234.50 元。", ".markdown": "# 合同\n金额 1234.50 元。", ".htm": "<html><body><article><p>合同金额 1234.50 元，不得退款。</p></article></body></html>", ".md": "# 合同\n\n金额 **1234.50** 元。",
                ".html": "<html><body><article><h1>合同</h1><p>合同金额 1234.50 元，不得退款。</p></article></body></html>",
                ".json": json.dumps({"金额": "1234.50", "说明": "不得退款"}, ensure_ascii=False),
                ".csv": "项目,金额\n合同,1234.50\n",
            }.items():
                path = root / ("sample" + suffix)
                path.write_text(text)
                segments = split_documents(convert_file(path), 256, 50)
                self.assertTrue(segments, suffix)
                self.assertTrue(all(segment["page_number"] is None for segment in segments), suffix)
            word = WordDocument()
            word.add_paragraph("合同金额 1234.50 元。")
            word.save(root / "sample.docx")
            slides = Presentation()
            slide = slides.slides.add_slide(slides.slide_layouts[6])
            slide.shapes.add_textbox(Inches(1), Inches(1), Inches(6), Inches(2)).text = "合同金额 1234.50 元。"
            slides.save(root / "sample.pptx")
            workbook = Workbook()
            workbook.active.title = "合同"
            workbook.active.append(["项目", "金额"])
            workbook.active.append(["0012", "1234.50"])
            workbook.save(root / "sample.xlsx")
            for suffix in [".docx", ".pptx", ".xlsx"]:
                documents = convert_file(root / ("sample" + suffix))
                self.assertIn("1234.50", "".join(doc.content or "" for doc in documents), suffix)
                segments = split_documents(documents, 256, 50)
                self.assertTrue(segments, suffix)
                if suffix == ".xlsx":
                    self.assertIn("0012", "".join(doc.content or "" for doc in documents))
                    self.assertEqual(segments[0]["source_label"], "合同")
            # 核验首尾及中间含空白页的 PDF 正文页码。
            from pypdf.generic import DecodedStreamObject, DictionaryObject, NameObject
            pdf = PdfWriter()
            for text in ["", "Contract page two", "", "Amount 1234.50 page four", ""]:
                page = pdf.add_blank_page(612, 792)
                if not text:
                    continue
                font = DictionaryObject({NameObject("/Type"): NameObject("/Font"), NameObject("/Subtype"): NameObject("/Type1"), NameObject("/BaseFont"): NameObject("/Helvetica")})
                page[NameObject("/Resources")] = DictionaryObject({NameObject("/Font"): DictionaryObject({NameObject("/F1"): pdf._add_object(font)})})
                stream = DecodedStreamObject()
                stream.set_data(f"BT /F1 12 Tf 50 700 Td ({text}) Tj ET".encode())
                page[NameObject("/Contents")] = pdf._add_object(stream)
            pdf.write(root / "sample.pdf")
            segments = split_documents(convert_file(root / "sample.pdf"), 256, 50)
            self.assertEqual([s["page_number"] for s in segments], [2, 4])

    def test_invalid_and_empty_files(self):
        """验证空白、损坏和加密原件的失败原因。"""
        with self.assertRaises(HTTPException) as empty:
            split_documents([Document(content=" \n\t")], 256, 50)
        self.assertEqual(empty.exception.detail["code"], "empty_content")
        self.assertEqual(empty.exception.detail["stage"], "splitting")
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "encrypted.pdf"
            writer = PdfWriter()
            writer.add_blank_page(612, 792)
            writer.encrypt("secret")
            writer.write(path)
            with self.assertRaises(HTTPException) as encrypted:
                convert_file(path)
            self.assertEqual(encrypted.exception.detail["code"], "encrypted_file")
            path = Path(directory) / "broken.json"
            path.write_text("{broken")
            with self.assertRaises(ValueError):
                convert_file(path)
