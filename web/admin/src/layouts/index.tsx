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
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { applyAppBrand, getAppBrandFromDetail } from '@/utils/appBrand';

const useAuth = (hasAuth: boolean) => {
  const { pathname } = useLocation();
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
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
    getUser().then(user => {
      Promise.all([
        user.role === ConstsUserRole.UserRoleAdmin
          ? getModel()
          : Promise.resolve(null),
        getKbList(),
      ]).then(([, kbList]) => {
        if (
          (user.role === ConstsUserRole.UserRoleUser ||
            user.role === ConstsUserRole.UserRoleManager) &&
          kbList.length === 0 &&
          pathname !== '/login'
        ) {
          navigate('401');
        }
      });
    });
  };

  useEffect(() => {
    if (hasAuth) {
      initData();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasAuth]);
};

const useAdminBrand = (hasAuth: boolean) => {
  const dispatch = useAppDispatch();
  const kbId = useAppSelector(state => state.config.kb_id);

  useEffect(() => {
    if (hasAuth && !kbId) return;

    const request = hasAuth
      ? getApiV1AppDetailCompat({ type: '1', kb_id: kbId })
      : getShareWebAppBrand();

    request
      .then(detail => {
        const brand = getAppBrandFromDetail(detail);
        dispatch(setAppBrand(brand));
        applyAppBrand(brand);
      })
      .catch(() => undefined);
  }, [dispatch, hasAuth, kbId]);
};

export const MainLayout = () => {
  useAuth(true);
  useAdminBrand(true);
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
  useAuth(hasAuth);
  useAdminBrand(hasAuth);
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
