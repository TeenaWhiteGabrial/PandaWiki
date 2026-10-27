'use client';
import { useStore } from '@/provider';
import { useMemo } from 'react';
import { getImagePath } from '@/utils/getImagePath';
import { useBasePath } from '@/hooks';

import Footer from '@panda-wiki/ui/footer';
import WelcomeFooterComponent from '@panda-wiki/ui/welcomeFooter';

export const FooterProvider = ({
  showBrand = false,
  isDocPage = false,
  isWelcomePage = false,
}: {
  showBrand?: boolean;
  isDocPage?: boolean;
  isWelcomePage?: boolean;
}) => {
  const { mobile = false, catalogWidth, kbDetail } = useStore();
  const basePath = useBasePath();
  const docWidth = useMemo(() => {
    if (isWelcomePage) return 'full';
    return kbDetail?.settings?.theme_and_style?.doc_width || 'full';
  }, [kbDetail, isWelcomePage]);
  const footerSetting = kbDetail?.settings?.footer_settings;
  const customStyle = kbDetail?.settings?.web_app_custom_style;

  return (
    <Footer
      mobile={mobile}
      catalogWidth={catalogWidth}
      showBrand={false}
      isDocPage={isDocPage}
      docWidth={docWidth}
      footerSetting={
        footerSetting
          ? {
              ...footerSetting,
              brand_logo: getImagePath(footerSetting?.brand_logo, basePath),
            }
          : undefined
      }
      customStyle={{
        ...customStyle,
        social_media_accounts: customStyle?.social_media_accounts?.map(
          (item: any) => ({
            ...item,
            icon: getImagePath(item.icon, basePath),
          }),
        ),
      }}
    />
  );
};

export const WelcomeFooter = ({
  showBrand = false,
}: {
  showBrand?: boolean;
}) => {
  const { mobile = false, catalogWidth, kbDetail } = useStore();
  const basePath = useBasePath();
  const footerSetting = kbDetail?.settings?.footer_settings;
  const customStyle = kbDetail?.settings?.web_app_custom_style;
  return (
    <WelcomeFooterComponent
      mobile={mobile}
      catalogWidth={catalogWidth}
      showBrand={false}
      isDocPage={false}
      docWidth='full'
      footerSetting={
        footerSetting
          ? {
              ...footerSetting,
              brand_logo: getImagePath(footerSetting?.brand_logo, basePath),
            }
          : undefined
      }
      customStyle={{
        ...customStyle,
        social_media_accounts: customStyle?.social_media_accounts?.map(
          (item: any) => ({
            ...item,
            icon: getImagePath(item.icon, basePath),
          }),
        ),
      }}
    />
  );
};

export default Footer;
