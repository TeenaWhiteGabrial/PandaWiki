"""Bounded Office parsing/rendering. No model code, shell execution or remote fetches."""
import ast
import math
import base64
import csv
import hashlib
import hmac
import io
import json
import os
import posixpath
from pathlib import PurePosixPath
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import zipfile
from office_style import style_document, style_presentation_body
from xml.etree import ElementTree as ET

MAX_BYTES = 20 * 1024 * 1024
MAX_EXPANDED = 100 * 1024 * 1024


def unpack(data, depth=0, budget=None):
    z = zipfile.ZipFile(io.BytesIO(data))
    infos = z.infolist()
    if budget is None:budget=[MAX_EXPANDED]
    budget[0]-=sum(i.file_size for i in infos)
    if budget[0]<0 or depth>1:raise ValueError('文件嵌套解压规模超限')
    if len(infos) > 4000 or sum(i.file_size for i in infos) > MAX_EXPANDED:
        raise ValueError('文件解压规模超限')
    for i in infos:
        p = PurePosixPath(i.filename)
        if p.is_absolute() or '..' in p.parts or '\\' in i.filename:
            raise ValueError('文件包路径非法')
        if i.file_size > 20 * 1024 * 1024 or i.file_size > max(i.compress_size, 1) * 300:
            raise ValueError('文件包压缩比例超限')
        low = i.filename.lower()
        if any(x in low for x in ('vbaproject', 'activex', 'externalLinks'.lower())):
            raise ValueError('不支持宏、嵌入附件或外部工作簿')
        if '/embeddings/' in low:
            if not low.endswith('.xlsx'):raise ValueError('不支持嵌入附件或 OLE 对象')
            embedded=unpack(z.read(i),depth+1,budget)
            if 'xl/workbook.xml' not in embedded.namelist():raise ValueError('嵌入数据格式无效')
        if low.endswith('.rels'):
            root = ET.fromstring(z.read(i))
            for rel in root:
                if rel.attrib.get('TargetMode') == 'External':
                    typ = rel.attrib.get('Type', '')
                    target = rel.attrib.get('Target', '')
                    if not typ.endswith('/hyperlink') or not re.match(r'^https?://[^\s]+$', target, re.I):
                        raise ValueError('文件包含外部素材或自动更新链接，请先移除')
    return z


def checked_data(raw):
    data = base64.b64decode(raw, validate=True)
    if len(data) > MAX_BYTES:
        raise ValueError('单文件不得超过 20 MB')
    return data


def clean_template_metadata(data):
    source=zipfile.ZipFile(io.BytesIO(data));out=io.BytesIO()
    def blocked(name):
        low=name.lower()
        return low.startswith(('customxml/','ppt/notesmasters/','ppt/handoutmasters/','ppt/notesslides/','ppt/comments/')) or low=='ppt/commentauthors.xml' or low=='docprops/custom.xml' or low.startswith('docprops/thumbnail')
    with zipfile.ZipFile(out,'w',zipfile.ZIP_DEFLATED) as target:
        for info in source.infolist():
            if blocked(info.filename):continue
            content=source.read(info)
            if info.filename.endswith('.rels'):
                root=ET.fromstring(content)
                base=posixpath.dirname(posixpath.dirname(info.filename))
                for rel in list(root):
                    dest=rel.attrib.get('Target','')
                    dest=posixpath.normpath(posixpath.join(base,dest)) if not dest.startswith('/') else dest.lstrip('/')
                    if blocked(dest):root.remove(rel)
                content=ET.tostring(root,encoding='utf-8',xml_declaration=True)
            elif info.filename=='[Content_Types].xml':
                root=ET.fromstring(content)
                for entry in list(root):
                    if blocked(entry.attrib.get('PartName','').lstrip('/')):root.remove(entry)
                content=ET.tostring(root,encoding='utf-8',xml_declaration=True)
            elif info.filename=='ppt/presentation.xml':
                root=ET.fromstring(content)
                for entry in list(root):
                    if entry.tag.rsplit('}',1)[-1] in ('notesMasterIdLst','handoutMasterIdLst'):root.remove(entry)
                content=ET.tostring(root,encoding='utf-8',xml_declaration=True)
            target.writestr(info.filename,content)
    return out.getvalue()


def template_copy(data):
    from pptx import Presentation
    p = Presentation(io.BytesIO(data))
    if abs(p.slide_width / p.slide_height - 16 / 9) > .03:
        raise ValueError('模板需为 16:9，请先调整幻灯片尺寸')
    layouts = []
    for idx, layout in enumerate(p.slide_layouts):
        kinds = {int(s.placeholder_format.type) for s in layout.placeholders}
        if kinds & {1, 3} and kinds & {2, 7}:
            layouts.append({'index': idx, 'name': layout.name})
    if not layouts:
        raise ValueError('模板缺少同时具有标题和正文占位符的版式，暂不支持自动套版')
    # Saving drops unreferenced slide/notes/comment parts. Master artwork remains;
    # master/layout sample text is cleared to avoid carrying old business text.
    for slide in list(p.slides._sldIdLst):
        p.part.drop_rel(slide.rId)
        p.slides._sldIdLst.remove(slide)
    for parent in [*p.slide_masters, *p.slide_layouts]:
        for shape in parent.shapes:
            if shape.has_text_frame:
                shape.text_frame.clear()
    cp = p.core_properties
    for key in ('author', 'last_modified_by', 'title', 'subject', 'comments', 'keywords', 'category', 'identifier'):
        setattr(cp, key, '')
    out = io.BytesIO()
    p.save(out)
    clean = clean_template_metadata(out.getvalue())
    unpack(clean)
    return clean, layouts


def raster(data):
    from PIL import Image
    p=Image.open(io.BytesIO(data))
    if p.format not in ('PNG','JPEG') or p.width*p.height>40000000 or max(p.size)>12000 or getattr(p,'n_frames',1)!=1:
        raise ValueError('图片需为单帧PNG/JPEG，像素不超过4000万且单边不超过12000')
    width,height,fmt=p.width,p.height,p.format;p.verify()
    p=Image.open(io.BytesIO(data));p.load();out=io.BytesIO()
    if fmt=='JPEG':p.convert('RGB').save(out,format='JPEG',quality=95)
    else:p.convert('RGBA').save(out,format='PNG')
    if out.tell()>MAX_BYTES:raise ValueError('图片规范化后过大，请压缩或缩小尺寸')
    out.seek(0);return out,width,height,fmt


def inspect(req):
    data = checked_data(req['data'])
    ext = req['format']
    text, report = '', {}
    result = {}
    if ext in ('docx', 'pptx', 'xlsx'):
        z = unpack(data)
        required = {'docx': 'word/document.xml', 'pptx': 'ppt/presentation.xml', 'xlsx': 'xl/workbook.xml'}[ext]
        if required not in z.namelist():
            raise ValueError('文件实际格式与后缀不符')
    if ext == 'pdf':
        from pypdf import PdfReader
        if not data.startswith(b'%PDF-'):
            raise ValueError('PDF 文件内容无效')
        p = PdfReader(io.BytesIO(data))
        if p.is_encrypted:
            raise ValueError('请上传未加密 PDF')
        if len(p.pages) > 300:
            raise ValueError('PDF 超过 300 页，请拆分上传')
        text = '\n'.join(f'[第{i+1}页]\n{page.extract_text() or ""}' for i, page in enumerate(p.pages))
        report['pages'] = len(p.pages)
        if not text.strip() or sum(len(page.extract_text() or '') for page in p.pages) < 20:
            raise ValueError('未提取到正文，可能为扫描件；请先 OCR 转换')
    elif ext == 'docx':
        from docx import Document
        d = Document(io.BytesIO(data))
        text = '\n'.join([p.text for p in d.paragraphs] + ['\t'.join(c.text for c in row.cells) for t in d.tables for row in t.rows])
    elif ext == 'pptx':
        from pptx import Presentation
        p = Presentation(io.BytesIO(data))
        if len(p.slides) > 100:
            raise ValueError('PPT 超过 100 页，请拆分')
        parts=[]
        for i,slide in enumerate(p.slides):
            body=[]
            for shape in slide.shapes:
                if shape.has_text_frame:body.append(shape.text)
                elif shape.has_table:body.extend('\t'.join(c.text for c in row.cells) for row in shape.table.rows)
                elif shape.has_chart:
                    chart=shape.chart
                    for series in chart.series:body.append(str(series.name)+': '+str(list(series.values)))
            parts.append(f'[幻灯片{i+1}]\n'+'\n'.join(body))
        text='\n'.join(parts)
        report['slides'] = len(p.slides)
        report['scope']='正文文本、表格与图表缓存数据；不执行嵌入内容'
        try:
            clean, layouts = template_copy(data)
            result['template'] = base64.b64encode(clean).decode()
            report.update(template_supported=True, layouts=layouts, template_notice='仅保留母版图片、配色与布局；原文字、备注、隐藏页不带入。请确认预览中的装饰图片。')
        except ValueError as e:
            report.update(template_supported=False, template_error=str(e))
    elif ext in ('xlsx', 'csv'):
        sheets = []
        if ext == 'xlsx':
            from openpyxl import load_workbook
            w = load_workbook(io.BytesIO(data), read_only=True, data_only=True, keep_links=False)
            if len(w.worksheets) > 5:
                raise ValueError('Excel 超过 5 个工作表，请拆分')
            for s in w.worksheets:
                rows = []
                for row in s.iter_rows(values_only=True):
                    if len(rows) >= 1001 or len(row) > 100:
                        raise ValueError('每表最多 1000 行数据、100 列')
                    rows.append([v if v is None or isinstance(v,(str,int,float,bool)) else str(v) for v in row])
                sheets.append({'name': s.title, 'rows': rows})
            w.close()
        else:
            rows = list(csv.reader(io.StringIO(data.decode('utf-8-sig'))))
            if len(rows) > 1001 or any(len(r) > 100 for r in rows):
                raise ValueError('CSV 超过 1000 行数据或 100 列')
            sheets.append({'name': '数据', 'rows': rows})
        text = json.dumps(sheets, ensure_ascii=False)
        report['sheets'] = [{'name': s['name'], 'rows': len(s['rows'])} for s in sheets]
    elif ext in ('png','jpg','jpeg'):
        normalized,width,height,fmt=raster(data)
        if fmt!=('PNG' if ext=='png' else 'JPEG'):raise ValueError('图片实际格式与文件后缀不符')
        report['image']={'width':width,'height':height,'format':fmt,'usage':'仅作为排版素材，不进行图像语义理解'}
        text=json.dumps(report['image'],ensure_ascii=False)
    elif ext == 'txt':
        text = data.decode('utf-8-sig')
        if '\x00' in text:
            raise ValueError('文本包含无效字符')
    else:
        raise ValueError('不支持的文件类型')
    if len(text) > 120000:
        raise ValueError('正文超过 12 万字符，请拆分资料；未截断使用')
    result.update(text=text, report=report, hash=hashlib.sha256(data).hexdigest())
    return result


def formula_results(columns, rows):
    cache, active = {}, set()
    def numeric(value):
        return isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value)
    def coords(ref):
        match = re.fullmatch(r'([A-Z]{1,3})([1-9][0-9]{0,3})', ref)
        if not match:
            raise ValueError('公式单元格引用无效')
        col = 0
        for c in match[1]: col = col * 26 + ord(c) - 64
        row, col = int(match[2]) - 1, col - 1
        if row > len(rows) or col >= len(columns):
            raise ValueError('公式引用超出本表数据范围')
        return row, col
    def cell(ref):
        ri, ci = coords(ref)
        if ri == 0: return columns[ci]
        key = (ri-1, ci)
        if key in cache: return cache[key]
        if key in active: raise ValueError('公式存在循环引用')
        value = rows[ri-1][ci]
        if isinstance(value, dict):
            if set(value) != {'formula'}: raise ValueError('公式对象格式无效')
            active.add(key)
            value = calculate(value['formula'])
            active.remove(key)
        cache[key] = value
        return value
    def aggregate(match):
        fn, raw = match[1], match[2]
        values = []
        for item in raw.split(','):
            if ':' in item:
                start, end = item.split(':')
                r1,c1 = coords(start); r2,c2 = coords(end)
                if r2<r1 or c2<c1: raise ValueError('公式区域方向无效')
                for ri in range(r1,r2+1):
                    for ci in range(c1,c2+1):
                        from xlsxwriter.utility import xl_rowcol_to_cell
                        values.append(cell(xl_rowcol_to_cell(ri,ci)))
            elif re.fullmatch(r'[A-Z]+[0-9]+',item): values.append(cell(item))
            else:
                try: values.append(float(item))
                except ValueError: raise ValueError('公式参数不支持')
        nums = [v for v in values if numeric(v)]
        if fn == 'COUNT': return str(len(nums))
        if fn == 'SUM': return repr(sum(nums))
        if not nums: raise ValueError('平均值/最值公式区域没有有效数字')
        return repr({'AVERAGE': lambda:sum(nums)/len(nums),'MIN':lambda:min(nums),'MAX':lambda:max(nums)}[fn]())
    def calculate(formula):
        if not isinstance(formula,str) or not formula.startswith('=') or len(formula)>256:
            raise ValueError('公式格式无效或超限')
        expr = re.sub(r'\s+', '', formula[1:]).upper()
        expr = re.sub(r'(SUM|AVERAGE|MIN|MAX|COUNT)\(([^()]*)\)',aggregate,expr)
        def replace(match):
            value=cell(match[0])
            if value is None or value=='': return '0'
            if not numeric(value): raise ValueError('算术公式引用了非数字单元格')
            return repr(value)
        expr = re.sub(r'[A-Z]{1,3}[1-9][0-9]{0,3}',replace,expr)
        tree = ast.parse(expr,mode='eval')
        if sum(1 for _ in ast.walk(tree))>150: raise ValueError('公式过于复杂')
        def walk(node):
            if isinstance(node,ast.Expression):return walk(node.body)
            if isinstance(node,ast.Constant) and numeric(node.value):return node.value
            if isinstance(node,ast.UnaryOp) and isinstance(node.op,(ast.UAdd,ast.USub)):
                v=walk(node.operand);return v if isinstance(node.op,ast.UAdd) else -v
            if isinstance(node,ast.BinOp) and isinstance(node.op,(ast.Add,ast.Sub,ast.Mult,ast.Div)):
                a,b=walk(node.left),walk(node.right)
                if isinstance(node.op,ast.Add):return a+b
                if isinstance(node.op,ast.Sub):return a-b
                if isinstance(node.op,ast.Mult):return a*b
                if b==0:raise ValueError('公式除数为零')
                return a/b
            raise ValueError('只支持基本四则运算及 SUM、AVERAGE、MIN、MAX、COUNT')
        value=walk(tree)
        if not numeric(value):raise ValueError('公式结果无效或超出数值范围')
        return round(value,12)
    for ri,row in enumerate(rows):
        for ci,value in enumerate(row):
            if isinstance(value,dict):
                from xlsxwriter.utility import xl_rowcol_to_cell
                cell(xl_rowcol_to_cell(ri+1,ci))
    return cache


def render(req):
    spec, fmt = req['spec'], req['format']
    title = str(spec.get('title', '生成文件'))[:100]
    preview_sheets=[]
    assets=req.get('assets',{})
    if not isinstance(assets,dict) or len(assets)>10:raise ValueError('图片素材数量超限')
    def image(ref):
        key=ref.get('upload_id')
        if key not in assets:raise ValueError('图片素材未获本轮授权')
        return raster(checked_data(assets[key]))
    out = io.BytesIO()
    if fmt == 'docx':
        from docx import Document
        from docx.shared import Pt
        d = Document()
        d.styles['Normal'].font.name = 'Noto Sans CJK SC'
        d.styles['Normal'].font.size = Pt(11)
        d.add_heading(title, 0)
        sections = spec.get('sections', [])
        if not sections or len(sections) > 60:
            raise ValueError('Word 章节数量无效')
        for sec in sections:
            d.add_heading(str(sec.get('title', '')), 1)
            for p in sec.get('paragraphs', []):
                d.add_paragraph(str(p))
            for p in sec.get('bullets', []):
                d.add_paragraph(str(p), 'List Bullet')
            for ref in sec.get('images',[]):
                from docx.shared import Inches
                stream,width,height,_=image(ref)
                # Fit to page content without changing aspect ratio.
                target=min(6.0,8.0*width/height)
                d.add_picture(stream,width=Inches(target))
                if ref.get('caption'):d.add_paragraph(str(ref['caption']))
            rows = sec.get('table', [])
            if rows:
                if len(rows) > 100 or max(map(len, rows)) > 15:
                    raise ValueError('Word 表格超限')
                t = d.add_table(rows=0, cols=max(map(len, rows)))
                t.style = 'Table Grid'
                for row in rows:
                    cells = t.add_row().cells
                    for c, val in zip(cells, row):
                        c.text = str(val)
        if sum(len(p.text) for p in d.paragraphs)+sum(len(c.text) for t in d.tables for row in t.rows for c in row.cells)>15000:raise ValueError('Word正文超过1.5万字，请拆分任务')
        style_document(d)
        d.save(out)
        from docx import Document as ReadDocument
        ReadDocument(io.BytesIO(out.getvalue()))
    elif fmt == 'pptx':
        from pptx import Presentation
        from pptx.util import Inches, Pt
        if req.get('template'):
            raw = checked_data(req['template'])
            unpack(raw)
            clean, _ = template_copy(raw)
            p = Presentation(io.BytesIO(clean))
        else:
            p = Presentation()
            p.slide_width, p.slide_height = Inches(13.333), Inches(7.5)
            for ph in p.slide_layouts[1].placeholders:
                if int(ph.placeholder_format.type) in (1,3):
                    ph.left,ph.top,ph.width,ph.height=Inches(.65),Inches(.4),p.slide_width-Inches(1.3),Inches(1.0)
                elif int(ph.placeholder_format.type) in (2,7):
                    ph.left,ph.top,ph.width,ph.height=Inches(.75),Inches(1.65),p.slide_width-Inches(1.5),p.slide_height-Inches(2.25)
        candidates = [l for l in p.slide_layouts if {int(s.placeholder_format.type) for s in l.placeholders} & {1, 3} and {int(s.placeholder_format.type) for s in l.placeholders} & {2, 7}]
        if not candidates:
            raise ValueError('模板无可用正文版式')
        slides = spec.get('slides', [])
        if not slides or len(slides) > 20:
            raise ValueError('PPT 需为 1～20 页')
        for slide_number, item in enumerate(slides, 1):
            slide = p.slides.add_slide(candidates[0])
            heading = str(item.get('title', ''))
            bullets = [str(v) for v in item.get('bullets', [])]
            if len(heading) > 50 or len(bullets) > 7 or sum(map(len, bullets)) > 420:
                raise ValueError('幻灯片文字过密，请减少文字或增加页面')
            slide.shapes.title.text = heading
            if not req.get('template'):
                from pptx.enum.text import PP_ALIGN
                from pptx.enum.shapes import MSO_SHAPE
                from pptx.dml.color import RGBColor
                title_shape=slide.shapes.title
                title_shape.left,title_shape.top=Inches(.65),Inches(.4)
                title_shape.width,title_shape.height=p.slide_width-Inches(1.3),Inches(1.0)
                for paragraph in title_shape.text_frame.paragraphs:
                    paragraph.alignment=PP_ALIGN.LEFT
                    paragraph.font.name,paragraph.font.size='Noto Sans CJK SC',Pt(30)
                    paragraph.line_spacing=Pt(36)
                    paragraph.space_before=paragraph.space_after=Pt(0)
                    for run in paragraph.runs:run.font.name='Noto Sans CJK SC';run.font.size=Pt(30);run.font.bold=True;run.font.color.rgb=RGBColor(30,58,100)
                stripe=slide.shapes.add_shape(MSO_SHAPE.RECTANGLE,0,0,p.slide_width,Inches(.07));stripe.fill.solid();stripe.fill.fore_color.rgb=RGBColor(40,75,140);stripe.line.fill.background()
            body = next(s for s in slide.placeholders if int(s.placeholder_format.type) in (2, 7))
            if not req.get('template'):
                body.left,body.top=Inches(.75),Inches(1.65)
                body.width,body.height=p.slide_width-Inches(1.5),p.slide_height-Inches(2.25)
            body.text_frame.clear()
            body.text_frame.word_wrap = True
            for i, txt in enumerate(bullets):
                paragraph = body.text_frame.paragraphs[0] if i == 0 else body.text_frame.add_paragraph()
                paragraph.text = txt
                for run in paragraph.runs:
                    if not req.get('template'):
                        run.font.name, run.font.size = 'Noto Sans CJK SC', Pt(20)
                if not req.get('template'):paragraph.space_after=Pt(14)
            if item.get('image'):
                stream,width,height,_=image(item['image']);left,top,area_width,area_height=body.left,body.top,body.width,body.height
                body._element.getparent().remove(body._element)
                scale=min(area_width/width,area_height/height);target_width,target_height=int(width*scale),int(height*scale)
                slide.shapes.add_picture(stream,left+(area_width-target_width)//2,top+(area_height-target_height)//2,width=target_width,height=target_height)
            elif item.get('table'):
                rows=item['table']
                if len(rows)>12 or not rows or not rows[0] or len(rows[0])>6 or any(len(r)!=len(rows[0]) for r in rows):raise ValueError('PPT 表格最多12行、6列且列数一致')
                area=(body.left,body.top,body.width,body.height)
                body._element.getparent().remove(body._element)
                table=slide.shapes.add_table(len(rows),len(rows[0]),*area).table
                for ri,row in enumerate(rows):
                    for ci,value in enumerate(row):
                        if len(str(value))>60:raise ValueError('PPT 表格文字过密')
                        table.cell(ri,ci).text=str(value)
                        for paragraph in table.cell(ri,ci).text_frame.paragraphs:
                            for run in paragraph.runs:run.font.size=Pt(16)
            elif item.get('chart'):
                from pptx.chart.data import CategoryChartData
                from pptx.enum.chart import XL_CHART_TYPE
                chart=item['chart'];categories=chart.get('categories',[]);series=chart.get('series',[])
                if not 1<=len(categories)<=12 or not 1<=len(series)<=4:raise ValueError('PPT 图表数据规模超限')
                data=CategoryChartData();data.categories=[str(v) for v in categories]
                for series_item in series:
                    values=series_item.get('values',[])
                    if len(values)!=len(categories) or any(not isinstance(v,(int,float)) or not math.isfinite(v) for v in values):raise ValueError('PPT 图表需提供完整真实数值')
                    data.add_series(str(series_item.get('name','数据')),values)
                area=(body.left,body.top,body.width,body.height);body._element.getparent().remove(body._element)
                kind={'column':XL_CHART_TYPE.COLUMN_CLUSTERED,'line':XL_CHART_TYPE.LINE,'pie':XL_CHART_TYPE.PIE}.get(chart.get('type','column'))
                if kind is None:raise ValueError('不支持的PPT图表类型')
                slide.shapes.add_chart(kind,*area,data)
            if not req.get('template') and not any(item.get(key) for key in ('image', 'table', 'chart')):
                style_presentation_body(p, slide, body, bullets, slide_number, len(slides))
            for shape in slide.shapes:
                if shape.left < 0 or shape.top < 0 or shape.left + shape.width > p.slide_width + 10000 or shape.top + shape.height > p.slide_height + 10000:
                    raise ValueError('模板元素超出页面边界，请调整模板')
        p.save(out)
        check = Presentation(io.BytesIO(out.getvalue()))
        if len(check.slides) != len(slides):
            raise ValueError('幻灯片完整性检查失败')
    elif fmt == 'xlsx':
        import xlsxwriter
        sheets = spec.get('sheets', [])
        sheet_limit = 25 if spec.get('catalog_export') is True else 5
        if not sheets or len(sheets) > sheet_limit:
            raise ValueError('Excel 工作表数量超限')
        w = xlsxwriter.Workbook(out, {'in_memory': True, 'strings_to_formulas': False, 'strings_to_urls': False})
        header = w.add_format({'bold': True, 'bg_color': '#E8EFFA', 'border': 1})
        seen = set()
        for i, s in enumerate(sheets):
            name = re.sub(r'[\[\]:*?/\\]', '_', str(s.get('name', f'数据{i+1}')))[:31].strip("'") or f'数据{i+1}'
            if name.casefold() in seen:
                raise ValueError('工作表名称重复')
            seen.add(name.casefold())
            sheet = w.add_worksheet(name)
            columns, rows = s.get('columns', []), s.get('rows', [])
            if not columns or len(columns) > 100 or len(rows) > 1000:
                raise ValueError('工作表列数或行数超限')
            if any(len(row)!=len(columns) for row in rows):raise ValueError('数据行列数不一致')
            values=formula_results(columns,rows)
            sheet.write_row(0, 0, [str(c) for c in columns], header)
            for ri, row in enumerate(rows, 1):
                if len(row) != len(columns):
                    raise ValueError('数据行列数不一致')
                for ci, value in enumerate(row):
                    if isinstance(value,dict):
                        sheet.write_formula(ri,ci,value['formula'],None,values[(ri-1,ci)])
                    elif isinstance(value,list):
                        raise ValueError('单元格必须为文本、数字、布尔、空值或受控公式')
                    else:
                        sheet.write(ri, ci, value)
            sheet.freeze_panes(1, 0)
            sheet.autofilter(0, 0, len(rows), len(columns)-1)
            sheet.set_column(0, len(columns)-1, 22)
        w.close()
        from openpyxl import load_workbook
        check = load_workbook(io.BytesIO(out.getvalue()), read_only=True, data_only=True)
        if len(check.worksheets) != len(sheets):
            raise ValueError('工作表完整性检查失败')
        for ws in check.worksheets:
            values=list(ws.iter_rows(values_only=True));preview_sheets.append({'name':ws.title,'columns':list(values[0]),'rows':[list(r) for r in values[1:]]})
        check.close()
    else:
        raise ValueError('不支持的生成格式')
    data = out.getvalue()
    unpack(data)
    return {'data': base64.b64encode(data).decode(), 'hash': hashlib.sha256(data).hexdigest(), 'size': len(data), 'title': title, 'preview_sheets':preview_sheets}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass

    def do_GET(self):
        if self.path == '/health':
            self.send_json(200, {'ok': True})
        else:
            self.send_json(404, {'error': 'not found'})

    def do_POST(self):
        token = os.environ.get('DOCUMENT_GENERATOR_TOKEN', '')
        if not token or not hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + token):
            self.send_json(401, {'error': 'Unauthorized'})
            return
        try:
            length = int(self.headers.get('Content-Length', 0))
            if not 0 < length < 32 * 1024 * 1024:
                raise ValueError('请求体超限')
            req = json.loads(self.rfile.read(length))
            if self.path == '/inspect':
                result = inspect(req)
            elif self.path == '/render':
                result = render(req)
            else:
                self.send_json(404, {'error': 'not found'})
                return
            self.send_json(200, result)
        except (ValueError, KeyError, TypeError, zipfile.BadZipFile, ET.ParseError) as e:
            self.send_json(422, {'error': str(e)[:240]})
        except Exception:
            self.send_json(422, {'error': '文件解析或生成失败，请检查文件内容和模板'})

    def send_json(self, status, value):
        data = json.dumps(value, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)


if __name__ == '__main__':
    ThreadingHTTPServer(('0.0.0.0', 8091), Handler).serve_forever()
