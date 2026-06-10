'use client';
import React from 'react';
import dayjs from 'dayjs';
import Watermark, { WatermarkProps } from './index';
import { useStore } from '@/provider';
import { ConstsWatermarkSetting } from '@/request/types';

const WaterMarkProvider = (props: WatermarkProps) => {
  const { children, ...rest } = props;
  const { kbDetail, authInfo } = useStore();

  const content = kbDetail?.settings?.watermark_content;

  const enable =
    kbDetail?.settings?.watermark_setting !==
    ConstsWatermarkSetting.WatermarkDisabled;
  if (!enable) {
    return children;
  }
  const time = `${authInfo?.account ?? ''} ${dayjs().format('YYYY-MM-DD HH:mm:ss')}`;
  const contentLines = [time, ...(content?.split('\n') || [])];
  return (
    <Watermark
      {...rest}
      content={contentLines}
      opacity={
        kbDetail?.settings?.watermark_setting ===
        ConstsWatermarkSetting.WatermarkVisible
          ? 0.1
          : 0.01
      }
    >
      {children}
    </Watermark>
  );
};

export default WaterMarkProvider;
