"""Default Office typography; user-supplied PPT masters remain untouched."""
import math
import unicodedata


FONT = 'Noto Sans CJK SC'


def _text_units(text):
    return sum(1 if unicodedata.east_asian_width(c) in ('W', 'F') else .55 for c in text)


def style_presentation_body(presentation, slide, body, bullets, page, total):
    from pptx.util import Inches, Pt
    from pptx.dml.color import RGBColor
    from pptx.enum.shapes import MSO_SHAPE
    from pptx.enum.text import MSO_ANCHOR, PP_ALIGN

    # Cards are purely visual grouping. They add no invented labels, sequence,
    # icons or metrics to the facts supplied by the user/model.
    count = len(bullets)
    cards = 2 <= count <= 4 and sum(map(len, bullets)) <= 360
    columns = 2 if count in (2, 4) else 1
    rows = math.ceil(count / columns) if count else 1
    gap = Inches(.18)
    width = (body.width - gap * (columns - 1)) // columns
    height = (body.height - gap * (rows - 1)) // rows
    padding = Inches(.24)
    font_size = 22
    if cards:
        for font_size in (22, 20, 18):
            available_width = max(1, (width - 2 * padding) / 12700)
            available_height = max(1, (height - 2 * padding) / 12700)
            if all(math.ceil(_text_units(text) * font_size / available_width) * font_size * 1.25 <= available_height for text in bullets):
                break
        else:
            cards = False
    if cards:
        left, top = body.left, body.top
        body._element.getparent().remove(body._element)
        for index, text in enumerate(bullets):
            x = left + (index % columns) * (width + gap)
            y = top + (index // columns) * (height + gap)
            card = slide.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, x, y, width, height)
            card.adjustments[0] = .06
            for effect in card._element.xpath('./p:style/a:effectRef'):
                effect.set('idx', '0')
            card.fill.solid()
            card.fill.fore_color.rgb = RGBColor(242, 246, 252)
            card.line.fill.background()
            frame = card.text_frame
            frame.word_wrap = True
            frame.vertical_anchor = MSO_ANCHOR.TOP
            frame.margin_left = frame.margin_right = padding
            frame.margin_top = frame.margin_bottom = padding
            paragraph = frame.paragraphs[0]
            paragraph.alignment = PP_ALIGN.LEFT
            paragraph.font.name, paragraph.font.size = FONT, Pt(font_size)
            paragraph.text = text
            paragraph.space_before = paragraph.space_after = Pt(0)
            paragraph.line_spacing = Pt(font_size * 1.25)
            for run in paragraph.runs:
                run.font.name, run.font.size = FONT, Pt(font_size)
                run.font.color.rgb = RGBColor(35, 51, 77)
    else:
        # Fit the supported list density without clipping or shrinking below
        # a readable size. Renderer limits still reject oversized content.
        available_width = max(1, body.width / 12700 - 30)
        available_height = body.height / 12700 - 10
        size = 18
        for candidate in (22, 20, 18):
            needed = sum(max(1, math.ceil(_text_units(t) * candidate / available_width)) * candidate * 1.25 for t in bullets) + max(0, count - 1) * 8
            if needed <= available_height:
                size = candidate
                break
        for paragraph in body.text_frame.paragraphs:
            paragraph.font.name, paragraph.font.size = FONT, Pt(size)
            paragraph.space_before = Pt(0)
            paragraph.space_after = Pt(8)
            paragraph.line_spacing = Pt(size * 1.25)
            for run in paragraph.runs:
                run.font.name, run.font.size = FONT, Pt(size)
                run.font.color.rgb = RGBColor(35, 51, 77)
    footer = slide.shapes.add_textbox(presentation.slide_width - Inches(1.4), presentation.slide_height - Inches(.4), Inches(.8), Inches(.22))
    footer.text_frame.margin_top = footer.text_frame.margin_bottom = 0
    paragraph = footer.text_frame.paragraphs[0]
    paragraph.font.name, paragraph.font.size = FONT, Pt(10)
    paragraph.line_spacing = Pt(12)
    paragraph.space_before = paragraph.space_after = Pt(0)
    paragraph.text = f'{page} / {total}'
    for run in paragraph.runs:
        run.font.name, run.font.size = FONT, Pt(10)
        run.font.color.rgb = RGBColor(100, 116, 139)


def style_document(document):
    from docx.shared import Cm, Pt, RGBColor
    from docx.oxml import OxmlElement
    from docx.oxml.ns import qn
    from docx.enum.text import WD_ALIGN_PARAGRAPH

    for section in document.sections:
        section.page_width, section.page_height = Cm(21), Cm(29.7)
        section.top_margin = section.bottom_margin = Cm(2.2)
        section.left_margin = section.right_margin = Cm(2.4)
        section.footer_distance = Cm(1.0)
        footer = section.footer.paragraphs[0]
        footer.alignment = WD_ALIGN_PARAGRAPH.RIGHT
        for text, field in [('第 ', None), ('', 'PAGE'), (' 页 / 共 ', None), ('', 'NUMPAGES'), (' 页', None)]:
            run = footer.add_run(text)
            run.font.size = Pt(9)
            run.font.color.rgb = RGBColor(100, 116, 139)
            if field:
                node = OxmlElement('w:fldSimple')
                node.set(qn('w:instr'), field)
                run._r.addnext(node)
    for name, size in [('Normal', 11), ('List Bullet', 11), ('Title', 26), ('Heading 1', 17), ('Heading 2', 14)]:
        style = document.styles[name]
        style.font.name, style.font.size = FONT, Pt(size)
        style.element.get_or_add_rPr().get_or_add_rFonts().set(qn('w:eastAsia'), FONT)
        style.paragraph_format.line_spacing = 1.35
        style.paragraph_format.space_after = Pt(7)
        if name in ('Title', 'Heading 1', 'Heading 2'):
            style.font.color.rgb = RGBColor(30, 58, 100)
            style.paragraph_format.keep_with_next = True
    for table in document.tables:
        if not table.rows:
            continue
        table.autofit = False
        width = document.sections[0].page_width - document.sections[0].left_margin - document.sections[0].right_margin
        weights = []
        for column in range(len(table.columns)):
            longest = max(_text_units(row.cells[column].text) for row in table.rows)
            weights.append(max(2, min(36, longest)) ** .7)
        for column, weight in enumerate(weights):
            column_width = int(width * weight / sum(weights))
            table.columns[column].width = column_width
            for row in table.rows:
                row.cells[column].width = column_width
        for row in table.rows:
            row._tr.get_or_add_trPr().append(OxmlElement('w:cantSplit'))
            for cell in row.cells:
                for paragraph in cell.paragraphs:
                    paragraph.paragraph_format.line_spacing = 1.15
                    paragraph.paragraph_format.space_after = Pt(4)
        properties = table.rows[0]._tr.get_or_add_trPr()
        properties.append(OxmlElement('w:tblHeader'))
        for cell in table.rows[0].cells:
            shade = OxmlElement('w:shd')
            shade.set(qn('w:fill'), 'EAF0FA')
            cell._tc.get_or_add_tcPr().append(shade)
            for paragraph in cell.paragraphs:
                for run in paragraph.runs:
                    run.bold = True
