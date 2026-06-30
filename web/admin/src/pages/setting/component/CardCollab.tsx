import {
  CollabMode,
  getApiV1CollabSetting,
  putApiV1CollabSetting,
} from '@/request/collabSetting';
import { message } from '@/components/ui';
import {
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useEffect, useState } from 'react';
import { FormItem, SettingCardItem } from './Common';

const DEFAULT_MAX_DOC_BYTES = 2 * 1024 * 1024;

const CardCollab = () => {
  const [mode, setMode] = useState<CollabMode>('disabled');
  const [maxDocBytes, setMaxDocBytes] = useState(DEFAULT_MAX_DOC_BYTES);
  const [isEdit, setIsEdit] = useState(false);

  useEffect(() => {
    getApiV1CollabSetting().then(res => {
      setMode(res.mode || 'disabled');
      setMaxDocBytes(res.max_doc_bytes || DEFAULT_MAX_DOC_BYTES);
    });
  }, []);

  const onSubmit = () => {
    putApiV1CollabSetting({
      mode,
      max_doc_bytes: maxDocBytes,
      block_min_bytes: 0,
      block_min_count: 0,
      block_window: 0,
    }).then(res => {
      setMode(res.mode || 'disabled');
      setMaxDocBytes(res.max_doc_bytes || DEFAULT_MAX_DOC_BYTES);
      setIsEdit(false);
      message.success('保存成功');
    });
  };

  return (
    <SettingCardItem title='协同编辑' isEdit={isEdit} onSubmit={onSubmit}>
      <FormItem
        label='协同模式'
        tooltip='开启后，仅富文本文档会按后端策略启用实时协同；Markdown 文档当前固定显示协同已暂停。'
      >
        <RadioGroup
          row
          value={mode}
          onChange={e => {
            setMode(e.target.value as CollabMode);
            setIsEdit(true);
          }}
        >
          <FormControlLabel
            value='prose_mirror'
            control={<Radio size='small' />}
            label='启用'
          />
          <FormControlLabel
            value='disabled'
            control={<Radio size='small' />}
            label='关闭'
          />
        </RadioGroup>
      </FormItem>
      <FormItem
        label='协同最大限制'
        tooltip='只有启用协同编辑且文档内容字节数不超过该限制时，富文本编辑页才会启用协同编辑。默认 2MB。'
      >
        <TextField
          type='number'
          size='small'
          value={maxDocBytes}
          slotProps={{ htmlInput: { min: 1024, step: 1024 } }}
          onChange={e => {
            setMaxDocBytes(Number(e.target.value) || DEFAULT_MAX_DOC_BYTES);
            setIsEdit(true);
          }}
        />
      </FormItem>
      <Stack gap={0.5}>
        <Typography variant='body2' color='text.secondary'>
          开启后，只有不超过协同最大限制的富文本文档才会启用实时协同；超限文档会显示协同已暂停。
        </Typography>
        <Typography variant='body2' color='text.secondary'>
          当前仅支持富文本协同，Markdown 文档协同已暂停。
        </Typography>
      </Stack>
    </SettingCardItem>
  );
};

export default CardCollab;
