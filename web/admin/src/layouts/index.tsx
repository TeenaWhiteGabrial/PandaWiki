import { Box } from '@mui/material';
import { Outlet, useLocation } from 'react-router-dom';
import Header from '@/components/Header';
import Sidebar from '@/components/Sidebar';
import { getApiV1AppDetailCompat } from '@/request/appCompat';
import { getApiV1ModelList } from '@/request/Model';
import { getApiV1KnowledgeBaseList } from '@/request/KnowledgeBase';
import { getShareWebAppBrand } from '@/request/shareAppBrand';
import { getApiV1UserCompat } from '@/request/userCompat';
import { useAppDispatch, useAppSelector } from '@/store';
import {
  setAppBrand,
  setModelStatus,
  setModelList,
  setKbList,
  setKbId,
  setUser,
} from '@/store/slices/config';
import { ConstsUserRole } from '@/request/types';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { applyAppBrand, getAppBrandFromDetail } from '@/utils/appBrand';
import { isAdministrator } from '@/utils/permission';
import { message } from '@/components/ui';

const useAuth = (hasAuth: boolean) => {
  const { pathname } = useLocation();
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const [authorized, setAuthorized] = useState(false);
  const getModel = () => {
    return getApiV1ModelList().then(res => {
      // @ts-expect-error 类型不匹配
      const chat = res.find(it => it.type === 'chat') || null;
      // @ts-expect-error 类型不匹配
      const embedding = res.find(it => it.type === 'embedding') || null;
      // @ts-expect-error 类型不匹配
      const rerank = res.find(it => it.type === 'rerank') || null;
      const status = chat && embedding && rerank;
      dispatch(setModelStatus(status));
      dispatch(setModelList(res));
      return status;
    });
  };

  const getKbList = (id?: string) => {
    const kb_id = id || localStorage.getItem('kb_id') || '';
    return getApiV1KnowledgeBaseList().then(res => {
      dispatch(setKbList(res));
      if (res.find(item => item.id === kb_id)) {
        dispatch(setKbId(kb_id));
      } else {
        dispatch(setKbId(res[0]?.id || ''));
      }
      return res;
    });
  };

  const getUser = () => {
    return getApiV1UserCompat().then(res => {
      dispatch(setUser(res));
      return res;
    });
  };

  const initData = () => {
    getUser()
      .then(user => {
        if (!isAdministrator(user.role)) {
          localStorage.removeItem('panda_wiki_token');
          dispatch(setUser({}));
          message.warning('仅管理员可登录管理后台，请使用用户端查阅文档');
          navigate('/login', { replace: true });
          return;
        }
        setAuthorized(true);
        Promise.all([
          user.role === ConstsUserRole.UserRoleAdmin
            ? getModel()
            : Promise.resolve(null),
          getKbList(),
        ]).then(([, kbList]) => {
          if (
            user.role === ConstsUserRole.UserRoleManager &&
            kbList.length === 0 &&
            pathname !== '/login'
          ) {
            navigate('401');
          }
        });
      })
      .catch(error => {
        setAuthorized(false);
        if (error?.status === 401) navigate('/login', { replace: true });
      });
  };

  useEffect(() => {
    if (hasAuth) {
      initData();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasAuth]);
  return authorized;
};

const useAdminBrand = (hasAuth: boolean, ready = true) => {
  const dispatch = useAppDispatch();
  const kbId = useAppSelector(state => state.config.kb_id);

  useEffect(() => {
    if (!ready) return;
    if (hasAuth && !kbId) return;

    const request = hasAuth
      ? getApiV1AppDetailCompat({ type: '1', kb_id: kbId })
      : getShareWebAppBrand();

    let active = true;
    request
      .then(detail => {
        if (!active) return;
        const brand = getAppBrandFromDetail(detail);
        dispatch(setAppBrand(brand));
        applyAppBrand(brand);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [dispatch, hasAuth, kbId, ready]);
};

export const MainLayout = () => {
  const authorized = useAuth(true);
  useAdminBrand(true, authorized);
  if (!authorized) return null;
  return (
    <>
      <Box
        sx={{
          position: 'relative',
          minWidth: '900px',
          minHeight: '100vh',
          fontSize: '16px',
          bgcolor: 'background.paper2',
        }}
      >
        <Sidebar />
        <Header />
        <Box
          sx={{
            pr: 2,
            width: 'calc(100% - 170px)',
            pt: '64px',
            ml: '170px',
            color: 'text.primary',
            'body.document-workbench-fullscreen &': {
              ml: 0,
              pr: 0,
              width: '100%',
            },
          }}
        >
          <Outlet />
        </Box>
      </Box>
    </>
  );
};

export const NoSidebarHeaderLayout = ({
  hasAuth = false,
}: {
  hasAuth: boolean;
}) => {
  const authorized = useAuth(hasAuth);
  useAdminBrand(hasAuth, !hasAuth || authorized);
  if (hasAuth && !authorized) return null;
  return (
    <Box
      sx={{
        minWidth: '900px',
      }}
    >
      <Outlet />
    </Box>
  );
};
