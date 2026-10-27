'use client';
import { useStore } from '@/provider';
import { redirect } from 'next/navigation';
import React from 'react';

import { deepSearchFirstNode } from '@/utils';
import { useBasePath } from '@/hooks';

const NodePage = () => {
  const basePath = useBasePath();
  const { tree, navList = [], lazyCatalog } = useStore();
  const entry = navList.find(n => n.entryNodeId)?.entryNodeId;
  if (lazyCatalog) return entry ? redirect(`${basePath}/node/${entry}`) : <></>;
  const firstNode = deepSearchFirstNode(tree || []);

  if (firstNode) {
    return redirect(`${basePath}/node/${firstNode.id}`);
  }

  return <></>;
};

export default NodePage;
