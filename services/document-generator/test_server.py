import base64
import io
import json
import unittest
import zipfile
from server import inspect, render, unpack


class GeneratorTests(unittest.TestCase):
    def test_default_ppt_groups_facts_without_losing_text(self):
        from pptx import Presentation
        bullets=['巡检人员集中填写检查结果。','异常生成工单，维修人员处理后回填结果。','管理者复盘未闭环事项。']
        result=render({'format':'pptx','spec':{'title':'合成巡检介绍','slides':[{'title':'巡检闭环','bullets':bullets}]}})
        deck=Presentation(io.BytesIO(base64.b64decode(result['data'])))
        texts=[shape.text for shape in deck.slides[0].shapes if shape.has_text_frame]
        for text in bullets:self.assertIn(text,texts)
        self.assertIn('1 / 1',texts)
        self.assertEqual(len(deck.slides),1)
        dense=['正文内容'*12 for _ in range(7)]
        dense_result=render({'format':'pptx','spec':{'title':'密集页面','slides':[{'title':'完整内容','bullets':dense}]}})
        dense_deck=Presentation(io.BytesIO(base64.b64decode(dense_result['data'])))
        dense_text='\n'.join(shape.text for shape in dense_deck.slides[0].shapes if shape.has_text_frame)
        self.assertEqual(dense_text.count(dense[0]),7)
        for shape in dense_deck.slides[0].shapes:
            if shape.has_text_frame:
                for paragraph in shape.text_frame.paragraphs:
                    if paragraph.text==dense[0]:
                        self.assertIsNotNone(paragraph.font.size)
                        self.assertEqual(paragraph.line_spacing,int(paragraph.font.size*1.25))

    def test_word_a4_chinese_and_repeating_table_header(self):
        from docx import Document
        from docx.shared import Cm
        result=render({'format':'docx','spec':{'title':'合成报告','sections':[{'title':'处理结果','paragraphs':['可验证的完整正文。'],'table':[['事项','状态'],['工单','已闭环']]}]}})
        document=Document(io.BytesIO(base64.b64decode(result['data'])))
        self.assertAlmostEqual(document.sections[0].page_width,Cm(21),delta=1000)
        self.assertIn('tblHeader',document.tables[0].rows[0]._tr.xml)
        self.assertIn('cantSplit',document.tables[0].rows[1]._tr.xml)
        self.assertIn('PAGE',document.sections[0].footer._element.xml)
        self.assertIn('可验证的完整正文', '\n'.join(p.text for p in document.paragraphs))

    def test_catalog_pagination_keeps_every_sheet(self):
        sheets=[{'name':f'目录{i+1}','columns':['名称'],'rows':[[f'资料{i+1}']]} for i in range(6)]
        with self.assertRaises(ValueError):
            render({'format':'xlsx','spec':{'title':'普通任务','sheets':sheets}})
        result=render({'format':'xlsx','spec':{'title':'目录册','catalog_export':True,'sheets':sheets}})
        self.assertEqual(len(result['preview_sheets']),6)
        self.assertEqual(result['preview_sheets'][-1]['rows'],[['资料6']])

    def test_template_removes_old_text_and_notes(self):
        original = render({'format':'pptx','spec':{'title':'模板','slides':[{'title':'PRIVATE_OLD_TITLE','bullets':['PRIVATE_OLD_BODY']}]}})
        parsed = inspect({'format':'pptx','data':original['data']})
        self.assertTrue(parsed['report']['template_supported'])
        clean = base64.b64decode(parsed['template'])
        z = zipfile.ZipFile(io.BytesIO(clean))
        self.assertFalse(any(n.startswith('ppt/slides/slide') for n in z.namelist()))
        self.assertNotIn(b'PRIVATE_OLD_BODY', b''.join(z.read(n) for n in z.namelist()))
        result = render({'format':'pptx','template':parsed['template'],'spec':{'title':'新演示','slides':[{'title':'NEW_TITLE','bullets':['NEW_BODY']}]}})
        reread = inspect({'format':'pptx','data':result['data']})
        self.assertIn('NEW_BODY',reread['text'])
        self.assertNotIn('PRIVATE_OLD_BODY',reread['text'])

    def test_spreadsheet_text_cannot_become_formula(self):
        from openpyxl import load_workbook
        result=render({'format':'xlsx','spec':{'title':'数据','sheets':[{'name':'数据','columns':['备注','值'],'rows':[['=HYPERLINK("https://example.com")',3],['+CMD',7]]}]}})
        w=load_workbook(io.BytesIO(base64.b64decode(result['data'])),data_only=False)
        self.assertEqual(w['数据']['A2'].data_type,'s')
        self.assertEqual(w['数据']['B3'].value,7)

    def test_reject_external_relationships_and_archive_paths(self):
        for name,xml in [('word/_rels/document.xml.rels',b'<Relationships><Relationship Type="image" TargetMode="External" Target="http://127.0.0.1/secret"/></Relationships>'),('../secret.txt',b'test')]:
            out=io.BytesIO()
            with zipfile.ZipFile(out,'w') as z:z.writestr(name,xml)
            with self.assertRaises(ValueError):unpack(out.getvalue())

    def test_no_silent_truncation(self):
        with self.assertRaises(ValueError):
            render({'format':'pptx','spec':{'title':'test','slides':[{'title':'test','bullets':['x'*421]}]}})
        with self.assertRaises(ValueError):
            inspect({'format':'txt','data':base64.b64encode(('中'*120001).encode()).decode()})

    def test_formula_cached_value_and_cycle_rejection(self):
        from openpyxl import load_workbook
        spec={'title':'公式','sheets':[{'name':'统计','columns':['项目','值'],'rows':[['A',3],['B',5],['合计',{'formula':'=SUM(B2:B3)'}],['平均',{'formula':'=AVERAGE(B2:B3)'}]]}]}
        result=render({'format':'xlsx','spec':spec})
        w=load_workbook(io.BytesIO(base64.b64decode(result['data'])),data_only=True)
        self.assertEqual(w['统计']['B4'].value,8)
        self.assertEqual(w['统计']['B5'].value,4)
        for formula in ['=1/0','=B2','=HYPERLINK("http://127.0.0.1")']:
            with self.assertRaises(ValueError):render({'format':'xlsx','spec':{'title':'非法公式','sheets':[{'name':'数据','columns':['键','值'],'rows':[['a',{'formula':formula}]]}]}})

    def test_authorized_raster_materials(self):
        from PIL import Image
        b=io.BytesIO();Image.new('RGB',(60,40),(60,100,160)).save(b,format='PNG')
        data=base64.b64encode(b.getvalue()).decode()
        metadata=inspect({'format':'png','data':data})
        self.assertEqual(metadata['report']['image']['width'],60)
        result=render({'format':'docx','assets':{'approved':data},'spec':{'title':'图片素材','sections':[{'title':'说明','images':[{'upload_id':'approved','caption':'用户提供的图片素材'}]}]}})
        z=zipfile.ZipFile(io.BytesIO(base64.b64decode(result['data'])))
        self.assertTrue(any(n.startswith('word/media/') for n in z.namelist()))
        with self.assertRaises(ValueError):render({'format':'docx','spec':{'title':'拒绝','sections':[{'title':'说明','images':[{'upload_id':'missing'}]}]}})

    def test_docx_chinese_and_table(self):
        result=render({'format':'docx','spec':{'title':'项目汇总','sections':[{'title':'成果','paragraphs':['测试正文'],'table':[['名称','数值'],['产品',12]]}]}})
        parsed=inspect({'format':'docx','data':result['data']})
        self.assertIn('测试正文',parsed['text'])
        self.assertIn('产品',parsed['text'])


if __name__=='__main__':unittest.main()
