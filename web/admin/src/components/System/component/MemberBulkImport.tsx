import { postApiV1UserCreate } from '@/request/User';
import {
  getApiV1EnterpriseDepartments,
  getApiV1EnterprisePermissionLevels,
  type Department,
  type PermissionLevel,
} from '@/request/Enterprise';
import { useAppSelector } from '@/store';
import { getApiV1UserListCompat } from '@/request/userCompat';
import type { V1CreateUserReq } from '@/request/types';
import { getAncestorIds } from '@/utils/enterprise';
import { message, Modal } from '@/components/ui';
import {
  Add,
  DeleteOutline,
  FileUploadOutlined,
  TableChartOutlined,
} from '@mui/icons-material';
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  IconButton,
  Tab,
  Tabs,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { Select as AntSelect } from 'antd';
import JSZip from 'jszip';
import { useEffect, useMemo, useRef, useState } from 'react';

type Role = 'manager' | 'user';
type InputMode = 'manual' | 'excel';

type ImportRow = {
  id: number;
  rowNumber: number;
  account: string;
  name: string;
  role: Role;
  roleError?: string;
  departmentPath: string;
  departmentId: string;
  permissionLevel?: string;
  permissionLevelId?: string;
  error?: string;
};

type ImportResult = {
  success: number;
  errors: { rowNumber: number; account: string; message: string }[];
};

const INITIAL_PASSWORD = 'YunzhouWiki@2026';
const MAX_IMPORT_ROWS = 500;
const MAX_FILE_SIZE = 5 * 1024 * 1024;
const HEADERS = ['账号', '姓名', '角色', '部门路径', '权限等级'];
const SPREADSHEET_NS =
  'http://schemas.openxmlformats.org/spreadsheetml/2006/main';
const OFFICE_REL_NS =
  'http://schemas.openxmlformats.org/officeDocument/2006/relationships';
const PACKAGE_REL_NS =
  'http://schemas.openxmlformats.org/package/2006/relationships';
const XLSX_MIME =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';

const parseCsv = (content: string): string[][] => {
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = '';
  let inQuotes = false;

  for (let index = 0; index < content.length; index += 1) {
    const char = content[index];
    if (inQuotes) {
      if (char === '"' && content[index + 1] === '"') {
        cell += '"';
        index += 1;
      } else if (char === '"') {
        inQuotes = false;
      } else {
        cell += char;
      }
      continue;
    }

    if (char === '"' && cell.length === 0) {
      inQuotes = true;
    } else if (char === ',') {
      row.push(cell);
      cell = '';
    } else if (char === '\n' || char === '\r') {
      if (char === '\r' && content[index + 1] === '\n') index += 1;
      row.push(cell);
      if (row.some(value => value.trim())) rows.push(row);
      row = [];
      cell = '';
    } else {
      cell += char;
    }
  }

  if (inQuotes) throw new Error('CSV 文件存在未闭合的引号');
  row.push(cell);
  if (row.some(value => value.trim())) rows.push(row);
  return rows;
};

const parseXml = (content: string) => {
  const document = new DOMParser().parseFromString(content, 'application/xml');
  if (document.getElementsByTagName('parsererror').length) {
    throw new Error('Excel 文件结构无效');
  }
  return document;
};

const resolveZipPath = (baseFile: string, target: string) => {
  const parts = target.startsWith('/') ? [] : baseFile.split('/').slice(0, -1);
  target.split('/').forEach(part => {
    if (!part || part === '.') return;
    if (part === '..') parts.pop();
    else parts.push(part);
  });
  return parts.join('/');
};

const columnIndexFromReference = (reference: string) => {
  const letters = reference.match(/^[A-Z]+/i)?.[0].toUpperCase() || '';
  return (
    [...letters].reduce(
      (index, letter) => index * 26 + letter.charCodeAt(0) - 64,
      0,
    ) - 1
  );
};

const parseExcel = async (file: File): Promise<string[][]> => {
  const zip = await JSZip.loadAsync(await file.arrayBuffer());
  const workbookFile = zip.file('xl/workbook.xml');
  const relationshipsFile = zip.file('xl/_rels/workbook.xml.rels');
  if (!workbookFile || !relationshipsFile) {
    throw new Error('无法读取 Excel 工作簿，请使用 .xlsx 格式');
  }

  const workbook = parseXml(await workbookFile.async('text'));
  const relationships = parseXml(await relationshipsFile.async('text'));
  const sheet = workbook
    .getElementsByTagNameNS(SPREADSHEET_NS, 'sheet')
    .item(0);
  const relationshipId =
    sheet?.getAttributeNS(OFFICE_REL_NS, 'id') || sheet?.getAttribute('r:id');
  if (!relationshipId) throw new Error('Excel 文件中没有工作表');

  const relationship = Array.from(
    relationships.getElementsByTagNameNS(PACKAGE_REL_NS, 'Relationship'),
  ).find(item => item.getAttribute('Id') === relationshipId);
  const target = relationship?.getAttribute('Target');
  if (!target) throw new Error('无法读取 Excel 的第一个工作表');

  const sheetPath = resolveZipPath('xl/workbook.xml', target);
  const sheetFile = zip.file(sheetPath);
  if (!sheetFile) throw new Error('Excel 的第一个工作表不存在');
  const worksheet = parseXml(await sheetFile.async('text'));

  const sharedStrings: string[] = [];
  const sharedStringsFile = zip.file('xl/sharedStrings.xml');
  if (sharedStringsFile) {
    const sharedStringsXml = parseXml(await sharedStringsFile.async('text'));
    Array.from(
      sharedStringsXml.getElementsByTagNameNS(SPREADSHEET_NS, 'si'),
    ).forEach(item => {
      const text = Array.from(item.getElementsByTagNameNS(SPREADSHEET_NS, 't'))
        .map(part => part.textContent || '')
        .join('');
      sharedStrings.push(text);
    });
  }

  const sheetData = worksheet
    .getElementsByTagNameNS(SPREADSHEET_NS, 'sheetData')
    .item(0);
  if (!sheetData) throw new Error('Excel 的第一个工作表为空');

  return Array.from(sheetData.getElementsByTagNameNS(SPREADSHEET_NS, 'row'))
    .map(row => {
      const values: string[] = [];
      Array.from(row.getElementsByTagNameNS(SPREADSHEET_NS, 'c')).forEach(
        (cell, index) => {
          const column = columnIndexFromReference(cell.getAttribute('r') || '');
          const position = column >= 0 ? column : index;
          const type = cell.getAttribute('t');
          let value = '';

          if (type === 'inlineStr') {
            value = Array.from(cell.getElementsByTagNameNS(SPREADSHEET_NS, 't'))
              .map(part => part.textContent || '')
              .join('');
          } else {
            const rawValue =
              cell.getElementsByTagNameNS(SPREADSHEET_NS, 'v').item(0)
                ?.textContent || '';
            value =
              type === 's' ? (sharedStrings[Number(rawValue)] ?? '') : rawValue;
          }
          values[position] = value;
        },
      );
      return values;
    })
    .filter(row => row.some(value => value?.trim()));
};

const xmlEscape = (value: string) =>
  value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');

const columnName = (index: number) => {
  let result = '';
  let value = index + 1;
  while (value > 0) {
    const remainder = (value - 1) % 26;
    result = String.fromCharCode(65 + remainder) + result;
    value = Math.floor((value - 1) / 26);
  }
  return result;
};

type TemplateSheet = {
  name: string;
  rows: string[][];
  widths: number[];
  frozenRows?: number;
  rowStyles?: Record<number, number>;
};

const TEMPLATE_STYLES_XML =
  `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
  `<styleSheet xmlns="${SPREADSHEET_NS}">` +
  `<fonts count="3"><font><sz val="11"/><color rgb="FF243247"/><name val="Microsoft YaHei"/></font><font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="Microsoft YaHei"/></font><font><b/><sz val="16"/><color rgb="FF1F4E78"/><name val="Microsoft YaHei"/></font></fonts>` +
  `<fills count="5"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF1F4E78"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFEAF1F8"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFE8F0FE"/><bgColor indexed="64"/></patternFill></fill></fills>` +
  `<borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border><border><left style="thin"><color rgb="FFD9E2F3"/></left><right style="thin"><color rgb="FFD9E2F3"/></right><top style="thin"><color rgb="FFD9E2F3"/></top><bottom style="thin"><color rgb="FFD9E2F3"/></bottom><diagonal/></border></borders>` +
  `<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
  `<cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf><xf numFmtId="0" fontId="2" fillId="4" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center" wrapText="1"/></xf><xf numFmtId="0" fontId="0" fillId="3" borderId="1" xfId="0" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center" wrapText="1"/></xf></cellXfs>` +
  `<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
  `</styleSheet>`;

const createWorksheetXml = (sheet: TemplateSheet) => {
  const lastColumn = Math.max(...sheet.rows.map(row => row.length), 1) - 1;
  const lastCell = `${columnName(lastColumn)}${sheet.rows.length}`;
  const columns = sheet.widths
    .map(
      (width, index) =>
        `<col min="${index + 1}" max="${index + 1}" width="${width}" customWidth="1"/>`,
    )
    .join('');
  const rowXml = sheet.rows
    .map((row, rowIndex) => {
      const style = sheet.rowStyles?.[rowIndex];
      const height = rowIndex === 0 && style === 2 ? 30 : style === 1 ? 25 : 34;
      return `<row r="${rowIndex + 1}" ht="${height}" customHeight="1">${row
        .map((value, columnIndex) => {
          const cellStyle = style === undefined ? '' : ` s="${style}"`;
          return `<c r="${columnName(columnIndex)}${rowIndex + 1}"${cellStyle} t="inlineStr"><is><t xml:space="preserve">${xmlEscape(value)}</t></is></c>`;
        })
        .join('')}</row>`;
    })
    .join('');
  const views = sheet.frozenRows
    ? `<sheetViews><sheetView workbookViewId="0"><pane ySplit="${sheet.frozenRows}" topLeftCell="A${sheet.frozenRows + 1}" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`
    : '<sheetViews><sheetView workbookViewId="0"/></sheetViews>';

  return (
    `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
    `<worksheet xmlns="${SPREADSHEET_NS}"><dimension ref="A1:${lastCell}"/>${views}<sheetFormatPr defaultRowHeight="22"/><cols>${columns}</cols><sheetData>${rowXml}</sheetData></worksheet>`
  );
};

const normalizeDepartmentPath = (value: string) =>
  value
    .split(/[/>]/)
    .map(part => part.trim())
    .filter(Boolean)
    .join('/');

const getDepartmentPath = (
  department: Department,
  departmentMap: Map<string, Department>,
) => {
  const names = [department.name];
  let parentId = department.parent_id;
  const visited = new Set([department.id]);
  while (parentId && departmentMap.has(parentId) && !visited.has(parentId)) {
    visited.add(parentId);
    const parent = departmentMap.get(parentId)!;
    names.unshift(parent.name);
    parentId = parent.parent_id;
  }
  return names.join('/');
};

const getErrorMessage = (error: unknown) => {
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === 'string' && message) return message;
  }
  if (error && typeof error === 'object' && 'response' in error) {
    const response = (error as { response?: unknown }).response;
    if (response && typeof response === 'object' && 'data' in response) {
      const data = (response as { data?: unknown }).data;
      if (data && typeof data === 'object' && 'message' in data) {
        const message = (data as { message?: unknown }).message;
        if (typeof message === 'string' && message) return message;
      }
    }
  }
  return '创建失败，请检查账号是否重复以及当前账号的部门权限';
};

const MemberBulkImport = ({ refresh }: { refresh: () => void }) => {
  const levelsDisabled = useAppSelector(
    state => state.config.user?.permission_levels_disabled,
  );
  const fileInputRef = useRef<HTMLInputElement>(null);
  const nextRowId = useRef(1);
  const departmentRequestStarted = useRef(false);
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<InputMode>('manual');
  const [departments, setDepartments] = useState<Department[]>([]);
  const [permissionLevels, setPermissionLevels] = useState<PermissionLevel[]>(
    [],
  );
  const [loadingDepartments, setLoadingDepartments] = useState(false);
  const [fileName, setFileName] = useState('');
  const [manualRows, setManualRows] = useState<ImportRow[]>([]);
  const [excelRows, setExcelRows] = useState<ImportRow[]>([]);
  const [reading, setReading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [progress, setProgress] = useState({ current: 0, total: 0 });
  const [result, setResult] = useState<ImportResult | null>(null);

  const departmentMap = useMemo(
    () => new Map(departments.map(department => [department.id, department])),
    [departments],
  );
  const departmentLookup = useMemo(() => {
    const byPath = new Map<string, Department>();
    const byName = new Map<string, Department[]>();
    const paths = departments.map(department => {
      const path = getDepartmentPath(department, departmentMap);
      byPath.set(normalizeDepartmentPath(path), department);
      const sameName = byName.get(department.name) || [];
      sameName.push(department);
      byName.set(department.name, sameName);
      return path;
    });
    return { byPath, byName, paths };
  }, [departments, departmentMap]);

  const rows = mode === 'manual' ? manualRows : excelRows;
  const setCurrentRows = mode === 'manual' ? setManualRows : setExcelRows;

  useEffect(() => {
    if (!open || departmentRequestStarted.current) return;
    departmentRequestStarted.current = true;
    setLoadingDepartments(true);
    Promise.all([
      getApiV1EnterpriseDepartments(),
      levelsDisabled
        ? Promise.resolve({ levels: [] })
        : getApiV1EnterprisePermissionLevels(),
    ])
      .then(([departmentResponse, levelResponse]) => {
        setDepartments(departmentResponse.departments || []);
        setPermissionLevels(
          (levelResponse.levels || []).filter(item => item.status === 'active'),
        );
      })
      .catch(() => {
        departmentRequestStarted.current = false;
        setDepartments([]);
        setPermissionLevels([]);
        message.error('读取部门或权限等级失败');
      })
      .finally(() => setLoadingDepartments(false));
  }, [open, levelsDisabled]);

  const resolveDepartment = (value: string) => {
    const normalized = normalizeDepartmentPath(value);
    if (departmentMap.has(value.trim())) return departmentMap.get(value.trim());
    const exactPath = departmentLookup.byPath.get(normalized);
    if (exactPath) return exactPath;
    const sameName = departmentLookup.byName.get(value.trim()) || [];
    return sameName.length === 1 ? sameName[0] : undefined;
  };

  const validateRows = (items: ImportRow[]) => {
    const seenAccounts = new Set<string>();
    return items.map(row => {
      const errors: string[] = [];
      const account = row.account.trim();
      const name = row.name.trim();
      const departmentPath = row.departmentPath.trim();
      const department = resolveDepartment(departmentPath);
      const permissionLevel = row.permissionLevel?.trim() || '';
      const matchingLevels = permissionLevels.filter(
        item => item.id === permissionLevel || item.name === permissionLevel,
      );
      const level = matchingLevels.length === 1 ? matchingLevels[0] : undefined;

      if (!account) errors.push('账号不能为空');
      else if (!/^[A-Za-z0-9_]+$/.test(account)) {
        errors.push('账号仅支持英文字母（大小写）、数字和下划线');
      }
      if (!name) errors.push('姓名不能为空');
      if (row.roleError) errors.push(row.roleError);
      if (!departmentPath) errors.push('请选择或填写部门路径');
      else if (!department) errors.push('未找到部门，请填写有效的完整路径');
      if (!levelsDisabled && permissionLevel && !level) {
        errors.push('权限等级不存在、已停用或名称不唯一');
      }
      if (account && seenAccounts.has(account)) {
        errors.push('账号重复');
      }
      if (account) seenAccounts.add(account);

      return {
        ...row,
        account,
        name,
        departmentId: department?.id || '',
        permissionLevel: level?.id || permissionLevel,
        permissionLevelId: level?.id || '',
        error: errors.length ? errors.join('；') : undefined,
      };
    });
  };

  const downloadTemplate = async () => {
    const guideRows = [
      ['批量添加用户｜填写指引', ''],
      ['', ''],
      ['操作步骤', '说明'],
      [
        '1. 填写用户信息',
        '切换到“用户信息”页，从第 2 行开始每行填写 1 位用户；第 1 行表头请保留，不要改名或删除。',
      ],
      [
        '2. 核对部门路径',
        '部门必须已在系统中创建。可在“部门路径”页复制完整路径；部门名称重名时必须填写从上级到下级的完整路径。',
      ],
      [
        '3. 上传并检查',
        '在批量添加弹窗的“上传 Excel”页选择本文件。系统会先解析并展示预览；错误行需修正后才能提交。',
      ],
      [
        '4. 确认添加',
        '核对预览无误后，点击“确认无误，开始添加”。创建成功后可在用户列表中查看结果。',
      ],
      ['', ''],
      ['字段', '填写要求'],
      [
        '账号（必填）',
        '仅允许英文字母（大小写）、数字和下划线；不可与系统中已有账号重复。',
      ],
      ['姓名（必填）', '填写用户的真实姓名或便于识别的名称。'],
      [
        '角色（可选）',
        '填写“普通用户”或“管理员”；也接受 user 或 manager。留空时默认为普通用户。',
      ],
      [
        '部门路径（必填）',
        '填写“部门路径”页中的完整路径，例如：集团/研发部。路径分隔符可使用 / 或 >。',
      ],
      [
        '权限等级（可选）',
        '填写“权限等级”页中的启用等级名称或 ID；留空使用默认等级。未启用等级权限时不设置此项。',
      ],
      [
        '初始密码',
        `系统统一设置为 ${INITIAL_PASSWORD}，无需在表格中填写；请通知用户首次登录后立即修改。`,
      ],
      ['', ''],
      ['填写示例（仅供参考，不会导入）', '', '', ''],
      HEADERS,
      ['Zhang_San01', '张三', '普通用户', '集团/研发部', ''],
      ['', ''],
      [
        '注意',
        `单次最多添加 ${MAX_IMPORT_ROWS} 人；上传文件不能超过 5 MB。导入时只读取第一个“用户信息”工作表，其他说明页不会创建用户。`,
      ],
    ];
    const guideRowStyles: Record<number, number> = Object.fromEntries(
      guideRows.map((row, index) => [
        index,
        index === 0 || row[0].startsWith('填写示例')
          ? 2
          : ['操作步骤', '字段', '账号'].includes(row[0])
            ? 1
            : 3,
      ]),
    );

    const sheets: TemplateSheet[] = [
      {
        name: '用户信息',
        rows: [HEADERS],
        widths: [22, 22, 20, 48, 24],
        frozenRows: 1,
        rowStyles: { 0: 1 },
      },
      {
        name: '填写指引',
        rows: guideRows,
        widths: [28, 72, 20, 44, 24],
        rowStyles: guideRowStyles,
      },
      {
        name: '部门路径',
        rows: [
          ['可填写的部门路径'],
          ...departmentLookup.paths.map(path => [path]),
        ],
        widths: [72],
        frozenRows: 1,
        rowStyles: Object.fromEntries(
          Array.from(
            { length: departmentLookup.paths.length + 1 },
            (_, index) => [index, index === 0 ? 1 : 3],
          ),
        ),
      },
    ];
    if (!levelsDisabled) {
      sheets.push({
        name: '权限等级',
        rows: [
          ['可填写的权限等级', '等级 ID', '默认等级'],
          ...permissionLevels.map(item => [
            item.name,
            item.id,
            item.is_default ? '是' : '',
          ]),
        ],
        widths: [28, 44, 18],
        frozenRows: 1,
        rowStyles: { 0: 1 },
      });
    }
    const zip = new JSZip();
    zip.file(
      '[Content_Types].xml',
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>${sheets
        .map(
          (_, index) =>
            `<Override PartName="/xl/worksheets/sheet${index + 1}.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`,
        )
        .join('')}</Types>`,
    );
    zip.file(
      '_rels/.rels',
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="${PACKAGE_REL_NS}"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
    );
    zip.file(
      'xl/workbook.xml',
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="${SPREADSHEET_NS}" xmlns:r="${OFFICE_REL_NS}"><bookViews><workbookView xWindow="0" yWindow="0" windowWidth="24000" windowHeight="12000" activeTab="1"/></bookViews><sheets>${sheets
        .map(
          (sheet, index) =>
            `<sheet name="${xmlEscape(sheet.name)}" sheetId="${index + 1}" r:id="rId${index + 1}"/>`,
        )
        .join('')}</sheets></workbook>`,
    );
    zip.file(
      'xl/_rels/workbook.xml.rels',
      `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="${PACKAGE_REL_NS}">${sheets
        .map(
          (_, index) =>
            `<Relationship Id="rId${index + 1}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet${index + 1}.xml"/>`,
        )
        .join(
          '',
        )}<Relationship Id="rId${sheets.length + 1}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
    );
    zip.file('xl/styles.xml', TEMPLATE_STYLES_XML);
    sheets.forEach((sheet, index) => {
      zip.file(
        `xl/worksheets/sheet${index + 1}.xml`,
        createWorksheetXml(sheet),
      );
    });

    const blob = await zip.generateAsync({ type: 'blob', mimeType: XLSX_MIME });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = '用户导入模板.xlsx';
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  const readCsv = async (file: File) => {
    const buffer = await file.arrayBuffer();
    try {
      return new TextDecoder('utf-8', { fatal: true }).decode(buffer);
    } catch {
      return new TextDecoder('gbk').decode(buffer);
    }
  };

  const handleFile = async (file?: File) => {
    if (!file) return;
    const extension = file.name.split('.').pop()?.toLowerCase();
    if (!['xlsx', 'csv'].includes(extension || '')) {
      message.error('请上传 .xlsx 或 .csv 文件');
      return;
    }
    if (file.size > MAX_FILE_SIZE) {
      message.error('文件不能超过 5 MB');
      return;
    }
    if (loadingDepartments || !departments.length) {
      message.error('当前没有可用部门，请先创建部门');
      return;
    }

    setReading(true);
    setResult(null);
    setExcelRows([]);
    setFileName(file.name);
    try {
      const matrix =
        extension === 'csv'
          ? parseCsv(await readCsv(file))
          : await parseExcel(file);
      if (matrix.length < 2) throw new Error('文件中没有可导入的数据行');
      if (matrix.length - 1 > MAX_IMPORT_ROWS) {
        throw new Error(`单次最多导入 ${MAX_IMPORT_ROWS} 个用户`);
      }

      const headers = matrix[0].map(value =>
        (value || '')
          .replace(/^\uFEFF/, '')
          .trim()
          .toLowerCase(),
      );
      const findHeaderIndex = (...names: string[]) =>
        headers.findIndex(header => names.includes(header));
      const accountIndex = findHeaderIndex('账号', 'account', 'username');
      const nameIndex = findHeaderIndex('姓名', 'name', 'real_name');
      const roleIndex = findHeaderIndex('角色', 'role');
      const permissionLevelIndex = findHeaderIndex(
        '权限等级',
        '等级',
        'permission_level',
        'permission_level_id',
      );
      const departmentIndex = findHeaderIndex(
        '部门路径',
        '部门',
        '所属部门',
        'department',
        'department_name',
      );

      if ([accountIndex, nameIndex, departmentIndex].some(index => index < 0)) {
        throw new Error(
          '表头必须包含：账号、姓名、部门路径；角色和权限等级列可选',
        );
      }

      const importedRows = matrix.slice(1).map((values, index): ImportRow => {
        const roleValue =
          roleIndex < 0 ? '' : (values[roleIndex] || '').trim().toLowerCase();
        let role: Role = 'user';
        let roleError: string | undefined;
        if (['manager', '管理员'].includes(roleValue)) role = 'manager';
        else if (roleValue && !['user', '普通用户'].includes(roleValue)) {
          roleError = '角色请填写“管理员”或“普通用户”';
        }

        return {
          id: index + 1,
          rowNumber: index + 2,
          account: values[accountIndex] || '',
          name: values[nameIndex] || '',
          role,
          roleError,
          departmentPath:
            departmentIndex < 0 ? '' : values[departmentIndex] || '',
          departmentId: '',
          permissionLevel:
            permissionLevelIndex < 0 ? '' : values[permissionLevelIndex] || '',
        };
      });
      nextRowId.current = importedRows.length + 1;
      setExcelRows(validateRows(importedRows));
    } catch (error) {
      setFileName('');
      message.error(getErrorMessage(error));
    } finally {
      setReading(false);
    }
  };

  const updateRow = (id: number, update: Partial<ImportRow>) => {
    setResult(null);
    setCurrentRows(current =>
      validateRows(
        current.map(row =>
          row.id === id
            ? {
                ...row,
                ...update,
                ...(update.role ? { roleError: undefined } : {}),
              }
            : row,
        ),
      ),
    );
  };

  const addManualRow = () => {
    if (manualRows.length >= MAX_IMPORT_ROWS) {
      message.warning(`单次最多添加 ${MAX_IMPORT_ROWS} 个用户`);
      return;
    }
    const id = nextRowId.current++;
    setManualRows(current =>
      validateRows([
        ...current,
        {
          id,
          rowNumber: id,
          account: '',
          name: '',
          role: 'user',
          departmentPath: '',
          departmentId: '',
        },
      ]),
    );
    setResult(null);
  };

  const removeRow = (id: number) => {
    setCurrentRows(current =>
      validateRows(current.filter(row => row.id !== id)),
    );
    setResult(null);
  };

  const validRows = rows.filter(row => !row.error);
  const canSubmit =
    rows.length > 0 &&
    validRows.length === rows.length &&
    !reading &&
    !loadingDepartments &&
    !importing;

  const resetImport = () => {
    setMode('manual');
    setFileName('');
    setManualRows([]);
    setExcelRows([]);
    setProgress({ current: 0, total: 0 });
    setResult(null);
    nextRowId.current = 1;
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const close = () => {
    if (importing) return;
    setOpen(false);
    if (!departments.length) departmentRequestStarted.current = false;
    resetImport();
  };

  const runImport = async () => {
    if (!canSubmit) return;
    setImporting(true);
    setProgress({ current: 0, total: validRows.length });
    const errors: ImportResult['errors'] = [];
    let success = 0;

    try {
      const existingAccounts = new Set<string>();
      const userPageSize = 100;
      const firstPage = await getApiV1UserListCompat({
        page: 1,
        page_size: userPageSize,
      });
      firstPage.users.forEach(user => {
        if (user.account) existingAccounts.add(user.account);
      });
      const totalPages = Math.ceil(firstPage.total / userPageSize);
      for (let page = 2; page <= totalPages; page += 1) {
        const userPage = await getApiV1UserListCompat({
          page,
          page_size: userPageSize,
        });
        userPage.users.forEach(user => {
          if (user.account) existingAccounts.add(user.account);
        });
      }

      for (let index = 0; index < validRows.length; index += 1) {
        const row = validRows[index];
        setProgress({ current: index + 1, total: validRows.length });
        if (existingAccounts.has(row.account)) {
          errors.push({
            rowNumber: row.rowNumber,
            account: row.account,
            message: '账号已存在',
          });
          continue;
        }

        const payload: V1CreateUserReq = {
          account: row.account,
          name: row.name,
          password: INITIAL_PASSWORD,
          role: row.role,
          permission_level_id: levelsDisabled
            ? undefined
            : row.permissionLevelId || undefined,
          department_ids: [
            ...new Set([
              ...getAncestorIds([row.departmentId], departments),
              row.departmentId,
            ]),
          ],
        };

        try {
          await postApiV1UserCreate(payload);
          existingAccounts.add(row.account);
          success += 1;
        } catch (error) {
          errors.push({
            rowNumber: row.rowNumber,
            account: row.account,
            message: getErrorMessage(error),
          });
        }
      }

      setResult({ success, errors });
      if (success) refresh();
      if (errors.length) {
        message.warning(
          `批量添加完成：成功 ${success} 条，失败 ${errors.length} 条`,
        );
      } else {
        message.success(`成功添加 ${success} 个用户`);
      }
    } catch (error) {
      message.error(`读取现有用户失败：${getErrorMessage(error)}`);
    } finally {
      setImporting(false);
    }
  };

  const openModal = () => {
    resetImport();
    setOpen(true);
  };

  const handleConfirm = () => {
    if (result) close();
    else void runImport();
  };

  const gridColumns = `42px minmax(120px, 1fr) minmax(100px, 1fr) 115px minmax(180px, 1.5fr) ${levelsDisabled ? '' : 'minmax(140px, 1fr) '}minmax(140px, 1.1fr) 42px`;

  return (
    <>
      <Button size='small' variant='outlined' onClick={openModal}>
        批量添加
      </Button>
      <input
        ref={fileInputRef}
        type='file'
        accept='.xlsx,.csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,text/csv'
        hidden
        onChange={event => {
          const file = event.target.files?.[0];
          event.target.value = '';
          void handleFile(file);
        }}
      />
      <Modal
        title='批量添加用户'
        open={open}
        width={levelsDisabled ? 1120 : 1280}
        onCancel={close}
        onOk={handleConfirm}
        maskClosable={!importing}
        closable={!importing}
        keyboard={!importing}
        okText={result ? '关闭' : importing ? '正在添加' : '确认无误，开始添加'}
        cancelText='取消'
        okButtonProps={{
          loading: importing,
          disabled: !result && !canSubmit,
        }}
      >
        <Stack gap={2}>
          <Alert severity='info'>
            <Stack gap={0.5}>
              <Box>
                可手动添加多行账号，或上传 .xlsx/.csv
                文件；上传后会先在本页面解析并展示，不会立即创建用户。
              </Box>
              <Box>
                每条记录需填写账号、姓名和部门路径；角色留空默认为普通用户。
                {!levelsDisabled && '可同时选择权限等级，留空使用默认等级。'}
              </Box>
              <Box>
                初始密码统一为 {INITIAL_PASSWORD}，请通知用户首次登录后修改。
              </Box>
            </Stack>
          </Alert>

          <Stack sx={{ borderBottom: 1, borderColor: 'divider' }}>
            <Tabs
              value={mode}
              onChange={(_, value: InputMode) => setMode(value)}
              aria-label='批量添加方式'
              textColor='primary'
              indicatorColor='primary'
            >
              <Tab
                value='manual'
                icon={<Add />}
                iconPosition='start'
                label='手动填写多个账号'
                disabled={importing}
                id='bulk-add-tab-manual'
                aria-controls='bulk-add-panel-manual'
              />
              <Tab
                value='excel'
                icon={<TableChartOutlined />}
                iconPosition='start'
                label='上传 Excel'
                disabled={importing}
                id='bulk-add-tab-excel'
                aria-controls='bulk-add-panel-excel'
              />
            </Tabs>
          </Stack>

          {mode === 'manual' ? (
            <Stack
              id='bulk-add-panel-manual'
              role='tabpanel'
              aria-labelledby='bulk-add-tab-manual'
              gap={1}
            >
              <Stack
                direction='row'
                justifyContent='space-between'
                alignItems='center'
              >
                <Typography variant='body2' color='text.secondary'>
                  一行填写一个账号，最多 {MAX_IMPORT_ROWS} 个。
                </Typography>
                <Button
                  size='small'
                  startIcon={<Add />}
                  onClick={addManualRow}
                  disabled={
                    importing || loadingDepartments || !departments.length
                  }
                >
                  添加一行
                </Button>
              </Stack>
              {renderRows(
                rows,
                gridColumns,
                updateRow,
                removeRow,
                departmentLookup.paths,
                importing,
                levelsDisabled ? undefined : permissionLevels,
              )}
            </Stack>
          ) : (
            <Stack
              id='bulk-add-panel-excel'
              role='tabpanel'
              aria-labelledby='bulk-add-tab-excel'
              gap={1}
            >
              <Stack direction='row' gap={1} alignItems='center'>
                <Button
                  size='small'
                  variant='outlined'
                  startIcon={<FileUploadOutlined />}
                  onClick={() => fileInputRef.current?.click()}
                  disabled={reading || importing || loadingDepartments}
                >
                  {reading ? '正在解析…' : '选择 Excel 文件'}
                </Button>
                {fileName && (
                  <Typography variant='body2' color='text.secondary' noWrap>
                    {fileName}
                  </Typography>
                )}
                <Button
                  size='small'
                  onClick={() => void downloadTemplate()}
                  disabled={loadingDepartments || importing}
                >
                  下载 Excel 模板
                </Button>
              </Stack>
              <Typography variant='body2' color='text.secondary'>
                只读取工作簿的第一个工作表。解析完成后可在下方检查并修改，再确认提交。
              </Typography>
              {renderRows(
                rows,
                gridColumns,
                updateRow,
                removeRow,
                departmentLookup.paths,
                importing,
                levelsDisabled ? undefined : permissionLevels,
              )}
            </Stack>
          )}

          {loadingDepartments && (
            <Typography variant='body2' color='text.secondary'>
              正在读取部门列表…
            </Typography>
          )}
          {rows.length > 0 && (
            <Stack
              direction='row'
              justifyContent='space-between'
              alignItems='center'
            >
              <Typography variant='body2' color='text.secondary'>
                当前 {rows.length} 条：{validRows.length} 条有效，
                {rows.length - validRows.length} 条需修正。
                {mode === 'excel' && ' 请检查预览内容后再确认添加。'}
              </Typography>
              {mode === 'excel' && (
                <Button
                  size='small'
                  color='inherit'
                  onClick={() => {
                    setExcelRows([]);
                    setFileName('');
                    setResult(null);
                  }}
                >
                  清空
                </Button>
              )}
            </Stack>
          )}

          {importing && (
            <Typography variant='body2' color='text.secondary'>
              正在添加 {progress.current}/{progress.total} 条…
            </Typography>
          )}
          {result && (
            <Alert severity={result.errors.length ? 'warning' : 'success'}>
              批量添加完成：成功 {result.success} 条，失败{' '}
              {result.errors.length} 条。
              {result.errors.length > 0 && (
                <Box sx={{ mt: 1, maxHeight: 180, overflowY: 'auto' }}>
                  {result.errors.slice(0, 50).map(error => (
                    <Typography
                      key={`${error.rowNumber}-${error.account}`}
                      variant='body2'
                    >
                      第 {error.rowNumber} 行 {error.account || '(账号为空)'}：
                      {error.message}
                    </Typography>
                  ))}
                </Box>
              )}
            </Alert>
          )}
        </Stack>
      </Modal>
    </>
  );
};

const renderRows = (
  rows: ImportRow[],
  gridColumns: string,
  updateRow: (id: number, update: Partial<ImportRow>) => void,
  removeRow: (id: number) => void,
  departmentPaths: string[],
  disabled: boolean,
  permissionLevels?: PermissionLevel[],
) => {
  if (!rows.length) {
    return (
      <Box
        sx={{
          p: 4,
          textAlign: 'center',
          border: '1px dashed',
          borderColor: 'divider',
          borderRadius: 1,
          color: 'text.secondary',
        }}
      >
        尚未添加数据
      </Box>
    );
  }

  const rowSx = {
    display: 'grid',
    gridTemplateColumns: gridColumns,
    alignItems: 'center',
    gap: 1,
    px: 1,
    py: 1,
  };

  return (
    <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
      <Box
        sx={{
          ...rowSx,
          position: 'sticky',
          top: 0,
          zIndex: 1,
          bgcolor: 'background.paper3',
          fontSize: 12,
          color: 'text.secondary',
        }}
      >
        <Box>#</Box>
        <Box>账号</Box>
        <Box>姓名</Box>
        <Box>角色</Box>
        <Box>部门路径</Box>
        {permissionLevels && <Box>权限等级</Box>}
        <Box>校验结果</Box>
        <Box />
      </Box>
      <Box sx={{ maxHeight: 360, overflow: 'auto' }}>
        {rows.map(row => (
          <Box
            key={row.id}
            sx={{
              ...rowSx,
              borderTop: '1px solid',
              borderColor: 'divider',
              bgcolor: row.error ? 'rgba(211, 47, 47, 0.035)' : undefined,
            }}
          >
            <Typography variant='body2' color='text.secondary'>
              {row.rowNumber}
            </Typography>
            <TextField
              size='small'
              value={row.account}
              placeholder='英文、数字、下划线'
              error={!!row.error?.includes('账号')}
              disabled={disabled}
              onChange={event =>
                updateRow(row.id, { account: event.target.value })
              }
            />
            <TextField
              size='small'
              value={row.name}
              placeholder='姓名'
              error={!!row.error?.includes('姓名')}
              disabled={disabled}
              onChange={event =>
                updateRow(row.id, { name: event.target.value })
              }
            />
            <AntSelect
              size='middle'
              value={row.role}
              onChange={role => updateRow(row.id, { role })}
              disabled={disabled}
              options={[
                { value: 'user', label: '普通用户' },
                { value: 'manager', label: '管理员' },
              ]}
            />
            <Autocomplete
              freeSolo
              size='small'
              options={departmentPaths}
              value={row.departmentPath}
              disabled={disabled}
              onChange={(_, value) =>
                updateRow(row.id, {
                  departmentPath: typeof value === 'string' ? value : '',
                })
              }
              onInputChange={(_, value, reason) => {
                if (reason === 'input' || reason === 'clear') {
                  updateRow(row.id, { departmentPath: value });
                }
              }}
              renderInput={params => (
                <TextField
                  {...params}
                  placeholder='选择或输入部门路径'
                  error={!!row.error?.includes('部门')}
                />
              )}
            />
            {permissionLevels && (
              <AntSelect
                value={row.permissionLevel || ''}
                disabled={disabled}
                status={row.error?.includes('权限等级') ? 'error' : undefined}
                onChange={permissionLevel =>
                  updateRow(row.id, { permissionLevel })
                }
                options={[
                  { value: '', label: '默认等级' },
                  ...permissionLevels.map(item => ({
                    value: item.id,
                    label: item.name,
                  })),
                ]}
              />
            )}
            <Typography
              variant='caption'
              color={row.error ? 'error' : 'success.main'}
              sx={{ overflowWrap: 'anywhere' }}
            >
              {row.error || '通过'}
            </Typography>
            <IconButton
              size='small'
              aria-label='删除此行'
              onClick={() => removeRow(row.id)}
              disabled={disabled}
            >
              <DeleteOutline fontSize='small' />
            </IconButton>
          </Box>
        ))}
      </Box>
    </Box>
  );
};

export default MemberBulkImport;
