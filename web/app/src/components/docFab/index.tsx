'use client';
import { useStore } from '@/provider';

const DocFab = () => {
  const { mobile } = useStore();
  if (mobile) return null;
  return null;
};

export default DocFab;
