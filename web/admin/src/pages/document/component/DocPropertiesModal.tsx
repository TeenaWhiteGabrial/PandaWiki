import type { DomainNodeListItemResp } from '@/request/types';
import { DomainNodeType } from '@/request/types';
import { useAppSelector } from '@/store';
import ReadingRulesModal from './ReadingRulesModal';

export default function DocPropertiesModal({
  open,
  onCancel,
  onOk,
  data,
}: {
  open: boolean;
  onCancel: () => void;
  onOk: () => void;
  isBatch?: boolean;
  data: DomainNodeListItemResp[];
}) {
  const kbID = useAppSelector(state => state.config.kb_id);
  return (
    <ReadingRulesModal
      open={open}
      kbID={kbID || ''}
      ids={data.map(item => item.id!)}
      names={data.map(item => item.name || '')}
      kind={
        data.every(item => item.type === DomainNodeType.NodeTypeDocument)
          ? '文档'
          : '目录'
      }
      canConfigure={
        data.length > 0 && data.every(item => item.can_manage === true)
      }
      onClose={onCancel}
      onSuccess={onOk}
    />
  );
}
