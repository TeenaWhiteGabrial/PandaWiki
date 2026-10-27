import LazyCatalogProvider from '@/provider/LazyCatalogProvider';
import { getShareNodeTree } from '@/request/NodeTree';
import { parsePathname } from '@/utils';
import { getServerPathname } from '@/utils/getServerHeader';
import NodeClientLayoutShell from './NodeClientLayoutShell';

export default async function Layout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const pathname = await getServerPathname();
  const { page, id } = parsePathname(pathname);
  const groups = await getShareNodeTree({
    path_id: page === 'node' ? id : undefined,
  });
  return (
    <LazyCatalogProvider initialGroups={groups || []}>
      <NodeClientLayoutShell>{children}</NodeClientLayoutShell>
    </LazyCatalogProvider>
  );
}
