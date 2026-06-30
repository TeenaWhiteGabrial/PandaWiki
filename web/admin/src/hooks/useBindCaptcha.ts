import { message } from '@/components/ui';
import { useEffect, useRef, useState } from 'react';

export function useBindCaptcha(
  id: string,
  { init = false, businessId = '' }: { init: boolean; businessId?: string },
) {
  const captcha = useRef<any>({});
  const resolveRef = useRef<any>(null);
  const [load, setLoad] = useState(false);
  const [token, setToken] = useState<string>();
  const captchaScriptUrl = import.meta.env.VITE_CAPTCHA_SCRIPT_URL || '';
  const captchaBusinessId =
    import.meta.env.VITE_CAPTCHA_BUSINESS_ID || businessId;

  const initCaptcha = () => {
    captcha.current = new (window as any).SCaptcha({
      businessid: captchaBusinessId,
      action: 'pow',
      position: 'mask',
    });
    captcha.current!.bind(
      ('#' + id).replace(/:/g, '\\:'),
      (action: any, data: any) => {
        if (action === 'finished') {
          captcha.current.reset();
          if (data) {
            setToken(data);
            resolveRef.current(data);
          } else {
            message.error('验证失败');
          }
        }
      },
    );
    const oldStart = captcha.current.start.bind(captcha.current);
    captcha.current.start = (e: any) => {
      oldStart(e);
      return new Promise(resolve => {
        resolveRef.current = resolve;
      });
    };
  };

  const loadCaptcha = () => {
    if (!captchaScriptUrl) {
      message.warning('当前环境未配置验证码服务');
      return;
    }
    if (document.querySelector(`script[src="${captchaScriptUrl}"]`)) {
      setLoad(true);
      return;
    }
    const script = document.createElement('script');
    script.src = captchaScriptUrl;
    document.body.appendChild(script);
    script.onload = () => {
      setLoad(true);
    };
  };

  useEffect(() => {
    if (init) {
      if (!load) {
        loadCaptcha();
      } else {
        initCaptcha();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [init, load]);

  return [captcha, token] as [any, string];
}
