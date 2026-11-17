import ReadingRulesModal from '@/pages/document/component/ReadingRulesModal';
import type { V1NavListResp } from '@/request/types';

export default function NavPermissionModal({
  open,
  nav,
  kb_id,
  onClose,
  onSuccess,
}: {
  open: boolean;
  nav: V1NavListResp | null;
  kb_id: string;
  onClose: () => void;
  onSuccess: () => void;
}) {
  return (
    <ReadingRulesModal
      open={open}
      kbID={kb_id}
      ids={nav?.id ? [nav.id] : []}
      names={[nav?.name || '']}
      kind='栏目'
      canConfigure={nav?.can_configure === true}
      onClose={onClose}
      onSuccess={onSuccess}
    />
  );
}
