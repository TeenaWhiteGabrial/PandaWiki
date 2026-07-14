import { formatMeta } from '@/utils';
import Doc from '@/views/node';
import { ResolvingMetadata } from 'next';

export const dynamic = 'force-dynamic';

export interface PageProps {
  params: Promise<{ id: string }>;
}

export async function generateMetadata(
  _props: PageProps,
  parent: ResolvingMetadata,
) {
  return await formatMeta({ title: '文档', description: '文档详情页' }, parent);
}

const DocPage = async ({ params }: PageProps) => {
  const { id = '' } = await params;
  return <Doc nodeId={id} />;
};

export default DocPage;
